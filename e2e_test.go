package main_test

import (
	"chat-v2/Middleware"
	"chat-v2/db"
	"chat-v2/handler"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/repository"
	"chat-v2/ws"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

var testRepo *repository.Repository
var suiteLockConn *pgxpool.Conn

const suiteLockKey int64 = 842020

type authResponse struct {
	Status    string `json:"status"`
	UserID    string `json:"user_id"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type conversationListResponse struct {
	Conversations []*db.Conversation `json:"conversations"`
}

func TestMain(m *testing.M) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to resolve test file path")
	}
	repoRoot := filepath.Dir(file)
	_ = godotenv.Load(filepath.Join(repoRoot, ".env"))
	_ = godotenv.Load(filepath.Join(repoRoot, "..", ".env"))
	logger.Init()

	dsn := os.Getenv("dbSource")
	if dsn == "" {
		panic("dbSource is not set")
	}
	if err := db.Connect(dsn); err != nil {
		panic("failed to connect to database: " + err.Error())
	}
	if err := db.DB.Ping(context.Background()); err != nil {
		panic("failed to ping database: " + err.Error())
	}

	lockConn, err := db.DB.Acquire(context.Background())
	if err != nil {
		panic("failed to acquire test DB lock connection: " + err.Error())
	}
	suiteLockConn = lockConn
	if _, err := suiteLockConn.Exec(context.Background(), `select pg_advisory_lock($1)`, suiteLockKey); err != nil {
		panic("failed to acquire test DB advisory lock: " + err.Error())
	}

	if err := helper.ResetSchema(); err != nil {
		panic("failed to reset database schema: " + err.Error())
	}
	testRepo = repository.NewRepository(db.DB)
	exitCode := m.Run()
	if suiteLockConn != nil {
		_, _ = suiteLockConn.Exec(context.Background(), `select pg_advisory_unlock($1)`, suiteLockKey)
		suiteLockConn.Release()
	}
	os.Exit(exitCode)
}

func TestChatFlow_E2E(t *testing.T) {
	reset := func() {
		if err := helper.ResetSchema(); err != nil {
			t.Fatalf("reset schema: %v", err)
		}
	}
	reset()

	maker, err := helper.NewJWTMaker("abcdefghijklmnopqrstuvwxyz123456")
	if err != nil {
		t.Fatalf("new jwt maker: %v", err)
	}
	hub := ws.NewHub()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()
	go hub.Run()

	authMiddleware := Middleware.JWTMiddleware(maker)
	mux := http.NewServeMux()
	mux.Handle("/signup", handler.SignUpHandler(testRepo))
	mux.Handle("/login", handler.LoginHandler(testRepo, maker))
	mux.Handle("/conversation/create", authMiddleware(handler.ConvCreateHandler(testRepo, maker)))
	mux.Handle("/conversation/list", authMiddleware(handler.ConvListHandler(testRepo, maker)))
	mux.Handle("/conversation/join", authMiddleware(handler.ConversationJoinHandler(testRepo, maker)))
	mux.Handle("/conversation/leave", authMiddleware(handler.ConversationLeaveHandler(testRepo, maker)))
	mux.Handle("/conversation/members", authMiddleware(handler.ConvMemberListHandler(testRepo, maker)))
	mux.Handle("/conversation/messages", authMiddleware(handler.MessageHandler(testRepo, maker)))
	server := httptest.NewServer(mux)
	defer server.Close()

	creator := signupAndLogin(t, server.URL, "e2e_creator")
	member := signupAndLogin(t, server.URL, "e2e_member")

	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/conversation/create", jsonBody(map[string]any{"title": "e2e conversation"}))
	createReq.Header.Set("Authorization", "Bearer "+creator.Token)
	createResp := doJSONRequest(t, createReq)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create conversation status=%d body=%s", createResp.StatusCode, createResp.Body)
	}

	listReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/list", nil)
	listReq.Header.Set("Authorization", "Bearer "+creator.Token)
	listResp := doJSONRequest(t, listReq)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list conversations status=%d body=%s", listResp.StatusCode, listResp.Body)
	}

	var convList conversationListResponse
	if err := json.Unmarshal(listResp.BodyBytes, &convList); err != nil {
		t.Fatalf("decode conversation list: %v", err)
	}
	if len(convList.Conversations) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(convList.Conversations))
	}
	conversationID := convList.Conversations[0].ID

	joinReq, _ := http.NewRequest(http.MethodPost, server.URL+"/conversation/join?conversation_id="+conversationID.String(), nil)
	joinReq.Header.Set("Authorization", "Bearer "+member.Token)
	joinResp := doJSONRequest(t, joinReq)
	if joinResp.StatusCode != http.StatusOK {
		t.Fatalf("join status=%d body=%s", joinResp.StatusCode, joinResp.Body)
	}

	membersReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/members?conversation_id="+conversationID.String(), nil)
	membersReq.Header.Set("Authorization", "Bearer "+creator.Token)
	membersResp := doJSONRequest(t, membersReq)
	if membersResp.StatusCode != http.StatusOK {
		t.Fatalf("members status=%d body=%s", membersResp.StatusCode, membersResp.Body)
	}
	var membersPayload map[string][]uuid.UUID
	if err := json.Unmarshal(membersResp.BodyBytes, &membersPayload); err != nil {
		t.Fatalf("decode members: %v", err)
	}
	if len(membersPayload["participants"]) != 2 {
		t.Fatalf("expected 2 participants, got %#v", membersPayload)
	}

	for i := 0; i < 3; i++ {
		message := &db.Message{
			ID:             uuid.New(),
			ConversationID: conversationID,
			SenderID:       creator.UserID,
			Content:        "seed message " + uuid.NewString(),
			CreatedAt:      time.Now().UTC().Add(-time.Duration(i) * time.Minute),
		}
		if err := testRepo.CreateMessage(context.Background(), message); err != nil {
			t.Fatalf("seed message %d: %v", i, err)
		}
	}

	repoPage, err := testRepo.GetMessagesByConversationID(context.Background(), conversationID, nil, 2)
	if err != nil {
		t.Fatalf("repo pagination check: %v", err)
	}
	if len(repoPage.Messages) != 2 || !repoPage.HasMore || repoPage.NextCursor == "" {
		t.Fatalf("unexpected repository pagination result: %#v", repoPage)
	}

	msgReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/messages?conversation_id="+conversationID.String()+"&limit=2", nil)
	msgReq.Header.Set("Authorization", "Bearer "+creator.Token)
	msgResp := doJSONRequest(t, msgReq)
	if msgResp.StatusCode != http.StatusOK {
		t.Fatalf("messages status=%d body=%s", msgResp.StatusCode, msgResp.Body)
	}
	var messages struct {
		Messages   []*db.Message `json:"messages"`
		NextCursor string        `json:"next_cursor"`
		HasMore    bool          `json:"has_more"`
	}
	if err := json.Unmarshal(msgResp.BodyBytes, &messages); err != nil {
		t.Fatalf("decode messages: %v", err)
	}
	if len(messages.Messages) != 2 {
		t.Fatalf("unexpected pagination response: %#v", messages)
	}

	nextReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/messages?conversation_id="+conversationID.String()+"&limit=2&before="+repoPage.NextCursor, nil)
	nextReq.Header.Set("Authorization", "Bearer "+creator.Token)
	nextResp := doJSONRequest(t, nextReq)
	if nextResp.StatusCode != http.StatusOK {
		t.Fatalf("next page status=%d body=%s", nextResp.StatusCode, nextResp.Body)
	}
	var nextMessages struct {
		Messages   []*db.Message `json:"messages"`
		NextCursor string        `json:"next_cursor"`
		HasMore    bool          `json:"has_more"`
	}
	if err := json.Unmarshal(nextResp.BodyBytes, &nextMessages); err != nil {
		t.Fatalf("decode next messages: %v", err)
	}
	if len(nextMessages.Messages) != 1 {
		t.Fatalf("unexpected second page response: %#v", nextMessages)
	}
}

type testUserSession struct {
	UserID uuid.UUID
	Token  string
}

type testHTTPResponse struct {
	StatusCode int
	Body       string
	BodyBytes  []byte
}

func signupAndLogin(t *testing.T, serverURL, usernamePrefix string) testUserSession {
	t.Helper()
	suffix := uuid.NewString()
	username := usernamePrefix + "_" + suffix[:8]
	email := username + "@example.com"
	password := "StrongPass123!"

	signupReq, _ := http.NewRequest(http.MethodPost, serverURL+"/signup", jsonBody(map[string]string{
		"username": username,
		"password": password,
		"email":    email,
	}))
	signupResp := doJSONRequest(t, signupReq)
	if signupResp.StatusCode != http.StatusCreated {
		t.Fatalf("signup status=%d body=%s", signupResp.StatusCode, signupResp.Body)
	}

	loginReq, _ := http.NewRequest(http.MethodPost, serverURL+"/login", jsonBody(map[string]string{
		"username": username,
		"password": password,
	}))
	loginResp := doJSONRequest(t, loginReq)
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginResp.StatusCode, loginResp.Body)
	}

	var payload authResponse
	if err := json.Unmarshal(loginResp.BodyBytes, &payload); err != nil {
		t.Fatalf("decode login payload: %v", err)
	}
	parsedUserID, err := uuid.Parse(payload.UserID)
	if err != nil {
		t.Fatalf("parse user id: %v", err)
	}
	return testUserSession{UserID: parsedUserID, Token: payload.Token}
}

func jsonBody(v any) *strings.Reader {
	data, _ := json.Marshal(v)
	return strings.NewReader(string(data))
}

func doJSONRequest(t *testing.T, req *http.Request) testHTTPResponse {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return testHTTPResponse{StatusCode: resp.StatusCode, Body: string(bodyBytes), BodyBytes: bodyBytes}
}
