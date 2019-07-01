package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/dgrijalva/jwt-go"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
)

func (h *Handler) Login(c echo.Context) error {
	RequestData := new(LoginParams)

	if err := c.Bind(RequestData); err != nil {
		return echo.ErrInternalServerError
	}

	if err := c.Validate(RequestData); err != nil {
		if useridValidation := CheckRegexp(`[^a-zA-Z0-9_]+`, RequestData.UserName); useridValidation {
			return echo.ErrUnauthorized
		}
	}

	UserID, Password, err := h.RoadPasswordAndUserID(RequestData.UserName)

	match, err := comparePasswordAndHash(RequestData.Password, Password)
	if err != nil {
		log.Println(err)
		return echo.ErrInternalServerError
	}
	if !match {
		return c.JSON(http.StatusUnauthorized, echo.Map{
			"status_code": "401",
		})
	}

	if err := h.UpdateAt(UserID); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status_code": "500",
		})
	}

	Token, err := GenerateJWTToken(UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status_code": "500",
		})
	}

	return c.JSON(http.StatusOK, echo.Map{
		"token": Token,
	})
}

func (h *Handler) Register(c echo.Context) error {

	RequestData := new(RegisterParams)
	User := new(RegisterParams)

	if err := c.Bind(RequestData); err != nil {
		return echo.ErrInternalServerError
	}
	if err := c.Validate(RequestData); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status_code": "400",
			"body": "invaild request",
		})
	}

	if useridValidate := CheckRegexp(`[^a-zA-Z0-9_]+`, RequestData.UserID); useridValidate {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status_code": "400",
			"body": "invaild userid",
		})
	}

	UserIDConflict, err := h.CheckUniqueUserID(strings.ToLower(RequestData.UserID))
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status_code": "500",
		})
	}
	if !UserIDConflict {
		return c.JSON(http.StatusConflict, echo.Map{
			"status_code": "409",
			"body": "conflict userid",
		})
	}
	User.UserID = strings.ToLower(RequestData.UserID)

	EMailConflict, err := h.CheckUniqueEmail(RequestData.EMail)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status_code": "500",
		})
	}
	if !EMailConflict {
		return c.JSON(http.StatusConflict, echo.Map{
			"status_code": "409",
			"body": "conflict mail address",
		})
	}
	User.EMail = RequestData.EMail

	// 参考サイト(MIT License):https://www.alexedwards.net/blog/how-to-hash-and-verify-passwords-with-argon2-in-go

	var p = &Argon2Params{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}

	User.Password, err = generatePassword(RequestData.Password, p)
	if err != nil {
		return echo.ErrInternalServerError
	}

	if err := h.InsertUserData(User); err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{
			"status_code": "500",
		})
	}

	url := fmt.Sprintf("/users/%s/", User.UserID)

	return c.JSON(http.StatusCreated, echo.Map{
		"status_code": "201",
		"account_url": url,
	})
}

func generatePassword(password string, p *Argon2Params) (encodedHash string, err error) {

	salt, err := generateRandomBytes(p.saltLength)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	encodedHash = fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, p.memory, p.iterations, p.parallelism, b64Salt, b64Hash)

	return encodedHash, nil
}

func generateRandomBytes(n uint32) ([]byte, error) {

	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func comparePasswordAndHash(password, encodedHash string) (match bool, err error) {
	p, salt, hash, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, p.keyLength)

	if subtle.ConstantTimeCompare(hash, otherHash) == 1 {
		return true, nil
	}
	return false, nil
}

func decodeHash(encodedHash string) (p *Argon2Params, salt, hash []byte, err error) {
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvaildHash
	}

	var version int
	if _, err := fmt.Sscanf(vals[2], "v=%d", &version); err != nil {
		return nil, nil, nil, err
	}

	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleVersion
	}

	p = &Argon2Params{}
	_, err = fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism)
	if err != nil {
		return nil, nil, nil, err
	}

	salt, err = base64.RawStdEncoding.DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	p.saltLength = uint32(len(salt))

	hash, err = base64.RawStdEncoding.DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	p.keyLength = uint32(len(hash))

	return p, salt, hash, nil
}

func CheckRegexp(reg, str string) bool {
	return regexp.MustCompile(reg).Match([]byte(str))
}

func (h *Handler) CheckUniqueUserID(UserID string) (bool, error) {
	var IsUnique sql.NullInt64
	db := h.DB

	BindParams := map[string]interface{}{
		"UserID": UserID,
	}

	Query, Params, err := sqlx.Named(
		`SELECT SUM(CASE WHEN user_id = :UserID THEN 1 ELSE 0 END) AS userid_count FROM users;`,
		BindParams,
	)
	if err != nil {
		return false, fmt.Errorf("Error CheckUniqueUserID(). Failed to set prepared statement: %s", err)
	}
	Rebind := db.Rebind(Query)

	if err := db.Get(&IsUnique, Rebind, Params...); err != nil {
		return false, fmt.Errorf("Error CheckUniqueUserID(). Failed to select user data: %s", err)
	}

	if IsUnique.Int64 == 0 {
		return true, nil
	}
	return false, nil
}

func (h *Handler) CheckUniqueEmail(EMail string) (bool, error) {
	var IsUnique sql.NullInt64
	db := h.DB
	BindParams := map[string]interface{}{
		"EMail": EMail,
	}
	Query, Params, err := sqlx.Named(
		`SELECT SUM(CASE WHEN user_id = :EMail THEN 1 ELSE 0 END) AS userid_count FROM users;`,
		BindParams,
	)
	if err != nil {
		return false, fmt.Errorf("Error CheckUniqueEMail(). Failed to set prepared statement: %s", err)
	}
	Rebind := db.Rebind(Query)

	if err := db.Get(&IsUnique, Rebind, Params...); err != nil {
		return false, fmt.Errorf("Error CheckUniqueEMail(). Failed to select user data: %s", err)
	}

	if IsUnique.Int64 == 0 {
		return true, nil
	}
	return false, nil
}

func (h *Handler) InsertUserData(User *RegisterParams) error {
	db := h.DB
	BindParams := map[string]interface{}{
		"UserID":     User.UserID,
		"EMail":      User.EMail,
		"ScreenName": User.ScreenName,
		"Now":        time.Now().Format(time.RFC3339Nano),
		"Password":   User.Password,
	}
	Query, Params, err := sqlx.Named(
		"INSERT INTO users (user_id, email, screen_name, created_at, updated_at, password) VALUES (:UserID, :EMail, :ScreenName, :Now, :Now, :Password)",
		BindParams,
	)
	if err != nil {
		return fmt.Errorf("Error InsertUserData(). Failed to set prepared statement: %s", err)
	}

	Rebind := db.Rebind(Query)

	if _, err := db.Exec(Rebind, Params...); err != nil {
		return fmt.Errorf("Error InsertUserData(). Failed to insert user data: %s", err)
	}
	return nil
}

func (h *Handler) RoadPasswordAndUserID(RequestUserID string) (string, string, error) {
	var UserID, Password string
	db := h.DB
	BindParams := map[string]interface{}{
		"UserID": RequestUserID,
	}
	Query, Params, err := sqlx.Named(
		"SELECT user_id, password FROM users WHERE user_id = :UserID OR email = :UserID",
		BindParams,
	)
	if err != nil {
		return "", "", fmt.Errorf("Error RoadPasswordAndUserID(). Failed to set prepared statement: %s", err)
	}

	Rebind := db.Rebind(Query)

	if db.QueryRowx(Rebind, Params...).Scan(&UserID, &Password); err != nil {
		return "", "", fmt.Errorf("Error RoadPasswordAndUserID(). Failed to select user data: %s", err)
	}
	return UserID, Password, nil
}

func (h *Handler) UpdateAt(RequestUserID string) error {
	db := h.DB
	BindParams := map[string]interface{}{
		"UserID": RequestUserID,
		"Now":    time.Now().Format(time.RFC3339Nano),
	}
	Query, Params, err := sqlx.Named(
		"UPDATE users SET updated_at = :Now WHERE user_id = :UserID",
		BindParams,
	)
	if err != nil {
		return fmt.Errorf("Error UpdateAt(). Failed to set prepared statement: %s", err)
	}

	Rebind := db.Rebind(Query)

	if _, err := db.Exec(Rebind, Params...); err != nil {
		return fmt.Errorf("Error UpdateAt(). Failed to update user data: %s", err)
	}
	return nil
}

func LoadPrivateKey() (*rsa.PrivateKey, error) {
	Key, err := ioutil.ReadFile("private-key.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to road private key: %s", err)
	}
	PrivateKey, err := jwt.ParseRSAPrivateKeyFromPEM(Key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Privatekey: %s", err)
	}
	return PrivateKey, nil
}

func GenerateJWTToken(UserID string) (string, error) {
	PrivateKey, err := LoadPrivateKey()
	if err != nil {
		return "", fmt.Errorf("LoadPrivateKey(): %s", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS512,
		&jwt.StandardClaims{
			ExpiresAt: time.Now().Add(time.Minute * 5).Unix(),
			IssuedAt:  time.Now().Unix(),
			NotBefore: time.Now().Add(time.Second * 5).Unix(),
			Audience:  UserID,
		},
	)
	t, err := token.SignedString(PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign string: %s", err)
	}
	return t, nil
}
