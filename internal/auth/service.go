package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/ayMissouri/noted/internal/storage/db"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUsernameTaken   = errors.New("that username is taken")
	ErrEmailTaken      = errors.New("an account with that email already exists")
	ErrInvalidUsername = errors.New("usernames are 3-32 characters: letters, digits, dot, dash, underscore")
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrWeakPassword    = errors.New("passwords must be at least 8 characters")
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$`)

type Service struct {
	sqldb *sql.DB
	q     *db.Queries
}

func NewService(sqldb *sql.DB) *Service {
	return &Service{sqldb: sqldb, q: db.New(sqldb)}
}

func (s *Service) CreateUser(ctx context.Context, username, email, password string, isAdmin bool) (db.User, error) {
	username = strings.TrimSpace(username)
	if !usernameRe.MatchString(username) {
		return db.User{}, ErrInvalidUsername
	}
	var emailPtr *string
	if email = strings.TrimSpace(email); email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return db.User{}, ErrInvalidEmail
		}
		emailPtr = &email
	}
	if len(password) < 8 || len(password) > 512 {
		return db.User{}, ErrWeakPassword
	}
	hash, err := HashPassword(password)
	if err != nil {
		return db.User{}, err
	}

	id, err := newID()
	if err != nil {
		return db.User{}, err
	}
	now := timestamp()
	admin := int64(0)
	if isAdmin {
		admin = 1
	}
	err = s.q.CreateUser(ctx, db.CreateUserParams{
		ID: id, Username: username, Email: emailPtr, PasswordHash: hash,
		IsAdmin: admin, CreatedAt: now, UpdatedAt: now,
	})
	switch uniqueColumn(err) {
	case "users.username":
		return db.User{}, ErrUsernameTaken
	case "users.email":
		return db.User{}, ErrEmailTaken
	}
	if err != nil {
		return db.User{}, fmt.Errorf("create user: %w", err)
	}
	return s.userByID(ctx, id)
}

func (s *Service) UserCount(ctx context.Context) (int64, error) {
	n, err := s.q.CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}

func (s *Service) UserByUsername(ctx context.Context, username string) (db.User, error) {
	u, err := s.q.GetUserByUsername(ctx, username)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, ErrUserNotFound
	}
	if err != nil {
		return db.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func (s *Service) userByID(ctx context.Context, id string) (db.User, error) {
	u, err := s.q.GetUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return db.User{}, ErrUserNotFound
	}
	if err != nil {
		return db.User{}, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

func uniqueColumn(err error) string {
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		msg := se.Error()
		for _, col := range []string{"users.username", "users.email"} {
			if strings.Contains(msg, col) {
				return col
			}
		}
	}
	return ""
}

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return id.String(), nil
}

func timestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
