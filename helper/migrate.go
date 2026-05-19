package helper

import(
	"context"
	"time"
	"os"
	"chat-v2/db"
)

func Migrate() error{
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schema, err := os.ReadFile("../db/0001_schema.up.sql")
	if err != nil {
		return err
	}

	_, err = db.GetDB().Exec(ctx, string(schema))
	if err != nil {
		return err
	}
	return nil
}

func Rollback() error{
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema, err := os.ReadFile("../db/0001_schema.down.sql")

	if err != nil {
		return err
	}

	_, err = db.GetDB().Exec(ctx, string(schema))
	if err != nil {
		return err
	}
	return nil
}