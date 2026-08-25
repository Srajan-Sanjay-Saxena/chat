package main_test

import (
	"chat-v2/config"
	"chat-v2/db"
	"chat-v2/db/redis"
	"chat-v2/handler"
	"chat-v2/helper"
	"chat-v2/logger"
	"chat-v2/middleware"
	"chat-v2/repository"

	// "chat-v2/ws"
	"chat-v2/internal/realtime"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var maker *helper.JWTMaker
var testRepo *repository.Repository
var suiteLockConn *pgxpool.Conn
var DB *pgxpool.Pool
var presenceStore *redis.PresenceStore

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
	logger.TestInit()
	logger.Log.Info("Starting test suite setup")

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Fatalf("configuration loading failed: %v", err)
	}

	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	DB, err := db.Connect(cfg.DBSource)

	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer func() {
		DB.Close()
	}()

	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS "%s"`, schema),
	)
	if err != nil {
		log.Fatalf("failed to create schema: %v", err)
	}

	if err := helper.ResetSchema(DB, schema); err != nil {
		log.Fatalf("failed to reset database schema: %v", err)
	}
	testRepo, err = repository.NewRepository(DB, schema)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	logger.Log.Info("Database setup complete")

	maker, err = helper.NewJWTMaker(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("failed to create JWT maker: %v", err)
	}

	// redis connection and presence store creation
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")
	redisDB := 0

	redisClient, err := redis.Connect(redisAddr, redisUsername, redisPassword, redisDB, false)
	if err != nil {
		logger.Log.Warn("Redis unavailable during test setup; continuing without Redis-backed presence", "error", err)
		redisClient = nil
	} else if redisClient != nil {
		defer redisClient.Close()
	}

	presenceStore = redis.NewPresenceStore(redisClient)

	exitCode := m.Run()

	_, err = DB.Exec(
		context.Background(),
		fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema),
	)
	if err != nil {
		logger.Log.Error("drop schema", "err", err)
	}
	os.Exit(exitCode)
}

func TestChatFlow_E2E(t *testing.T) {
	// reset := func() {
	// 	if err := helper.ResetSchema(DB); err != nil {
	// 		t.Fatalf("reset schema: %v", err)
	// 	}
	// }
	// reset()

	hub := realtime.NewHub()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()
	go hub.Run()

	h := &handler.Handler{
		Repo:  testRepo,
		Maker: maker,
	}

	authMiddleware := middleware.JWTMiddleware(maker)
	mux := http.NewServeMux()
	mux.Handle("/signup", h.SignUpHandler())
	mux.Handle("/login", h.LoginHandler())
	mux.Handle("/conversation/create", authMiddleware(h.ConvCreateHandler()))
	mux.Handle("/conversation/list", authMiddleware(h.ConvListHandler()))
	mux.Handle("/conversation/join", authMiddleware(h.ConversationJoinHandler()))
	mux.Handle("/conversation/leave", authMiddleware(h.ConversationLeaveHandler()))
	mux.Handle("/conversation/members", authMiddleware(h.ConvMemberListHandler()))
	mux.Handle("/conversation/messages", authMiddleware(h.MessageHandler()))
	server := httptest.NewServer(mux)
	defer server.Close()

	creator := signupAndLogin(t, server.URL, "e2e_creator")
	member := signupAndLogin(t, server.URL, "e2e_member")

	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/conversation/create", jsonBody(map[string]any{"title": "e2e conversation"}))
	createReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
	createResp := doJSONRequest(t, createReq)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create conversation status=%d body=%s", createResp.StatusCode, createResp.Body)
	}

	listReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/list", nil)
	listReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
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
	joinReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: member.Token,
		},
	)
	joinResp := doJSONRequest(t, joinReq)
	if joinResp.StatusCode != http.StatusOK {
		t.Fatalf("join status=%d body=%s", joinResp.StatusCode, joinResp.Body)
	}

	membersReq, _ := http.NewRequest(http.MethodGet, server.URL+"/conversation/members?conversation_id="+conversationID.String(), nil)
	membersReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
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
	msgReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
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
	nextReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
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

func TestE2E_CreatePrivateByUsernames(t *testing.T) {
	// reset := func() {
	// 	if err := helper.ResetSchema(DB); err != nil {
	// 		t.Fatalf("reset schema: %v", err)
	// 	}
	// }
	// reset()

	hub := realtime.NewHub()
	defer func() {
		hub.Stop()
		<-hub.Done()
	}()
	go hub.Run()

	h := &handler.Handler{
		Repo:  testRepo,
		Maker: maker,
	}

	authMiddleware := middleware.JWTMiddleware(maker)
	mux := http.NewServeMux()
	mux.Handle("/signup", h.SignUpHandler())
	mux.Handle("/login", h.LoginHandler())
	mux.Handle("/conversation/create", authMiddleware(h.ConvCreateHandler()))
	server := httptest.NewServer(mux)
	defer server.Close()

	creator := signupAndLogin(t, server.URL, "ec")
	member := signupAndLogin(t, server.URL, "em")
	// t.Logf("creator.token=%s", creator.Token)
	// Create private conversation by username (creator -> member)
	createReq, _ := http.NewRequest(http.MethodPost, server.URL+"/conversation/create", jsonBody(map[string]any{"type": "private", "participant_usernames": []string{member.Username}}))
	createReq.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
	createResp := doJSONRequest(t, createReq)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create private by usernames status=%d body=%s", createResp.StatusCode, createResp.Body)
	}
	var payload1 struct {
		Conversation db.Conversation `json:"conversation"`
		Created      bool            `json:"created"`
	}
	if err := json.Unmarshal(createResp.BodyBytes, &payload1); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	// Create again - should return created=false and same conversation id
	createReq2, _ := http.NewRequest(http.MethodPost, server.URL+"/conversation/create", jsonBody(map[string]any{"type": "private", "participant_usernames": []string{member.Username}}))
	createReq2.AddCookie(
		&http.Cookie{
			Name:  "access_token",
			Value: creator.Token,
		},
	)
	createResp2 := doJSONRequest(t, createReq2)
	if createResp2.StatusCode != http.StatusOK {
		t.Fatalf("second create expected 200 got=%d body=%s", createResp2.StatusCode, createResp2.Body)
	}
	var payload2 struct {
		Conversation db.Conversation `json:"conversation"`
		Created      bool            `json:"created"`
	}
	if err := json.Unmarshal(createResp2.BodyBytes, &payload2); err != nil {
		t.Fatalf("decode second create: %v", err)
	}
	if payload2.Created {
		t.Fatalf("expected created=false on duplicate create, got created=true")
	}
	if payload1.Conversation.ID != payload2.Conversation.ID {
		t.Fatalf("expected same conversation id on duplicate create: got %s vs %s", payload1.Conversation.ID, payload2.Conversation.ID)
	}
}

type testUserSession struct {
	UserID   uuid.UUID
	Token    string
	Username string
}

type testHTTPResponse struct {
	StatusCode int
	Body       string
	BodyBytes  []byte
	Response   *http.Response
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
	// t.Logf("login cookies: %+v", loginResp.Response.Cookies())
	var token string
	for _, cookie := range loginResp.Response.Cookies() {
		if cookie.Name == "access_token" {
			token = cookie.Value
			break
		}
	}

	return testUserSession{UserID: parsedUserID, Token: token, Username: username}
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
	// t.Logf("response: %v", resp)
	return testHTTPResponse{StatusCode: resp.StatusCode, Body: string(bodyBytes), BodyBytes: bodyBytes, Response: resp}
}
