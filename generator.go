package fireauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// Version used for creating token
	Version = 0
	// TokenSep used as a delimiter for the token
	TokenSep = "."
)

// Generic errors
var (
	ErrNoUIDKey           = errors.New(`Data payload must contain a "uid" key`)
	ErrUIDNotString       = errors.New(`Data payload key "uid" must be a string`)
	ErrUIDTooLong         = errors.New(`Data payload key "uid" must not be longer than 256 characters`)
	ErrEmptyDataNoOptions = errors.New("Data is empty and no options are set.  This token will have no effect on Firebase.")
	ErrTokenTooLong       = errors.New("Generated token is too long. The token cannot be longer than 1024 bytes.")
)

// Generator represents a token generator
type Generator struct {
	secret string
}

// Option represent the claims used when creating an authentication token
// https://www.firebase.com/docs/rest/guide/user-auth.html#section-rest-tokens-without-helpers
type Option struct {
	// NotBefote is the token "not before" date as a number of seconds since the Unix epoch.
	// If specified, the token will not be considered valid until after this date.
	NotBefore int64 `json:"nbf,omitempty"`

	// Expiration is the token expiration date as a number of seconds since the Unix epoch.
	// If not specified, by default the token will expire 24 hours after the "issued at" date (iat).
	Expiration int64 `json:"exp,omitempty"`

	// Admin when set to true to make this an "admin" token, which grants full read and
	// write access to all data.
	Admin bool `json:"admin,omitempty"`

	// Debug when set to true to enable debug mode, which provides verbose error messages
	// when Security and Firebase Rules fail.
	Debug bool `json:"debug,omitempty"`
}

// Data is used to create a token. The token data can contain any data of your choosing,
// however it must contain a `uid` key, which must be a string of less than 256 characters
type Data map[string]interface{}

// New creates a new Generator
func New(secret string) *Generator {
	return &Generator{
		secret: secret,
	}
}

// CreateToken generates a new token with the given Data and options
func (t *Generator) CreateToken(data Data, options *Option) (string, error) {
	// make sure we have valid parameters
	if data == nil && (options == nil || (!options.Admin && !options.Debug)) {
		return "", ErrEmptyDataNoOptions
	}

	// validate the data
	if err := validate(data, (options != nil && options.Admin)); err != nil {
		return "", err
	}

	// generate the encoded headers
	encodedHeader, err := encodedHeader()
	if err != nil {
		return "", err
	}

	// setup the claims for the token
	claim := struct {
		*Option
		Version  int   `json:"v"`
		Data     Data  `json:"d"`
		IssuedAt int64 `json:"iat"`
	}{
		Option:   options,
		Version:  Version,
		Data:     data,
		IssuedAt: time.Now().Unix(),
	}

	// generate the encoded claims
	claimBytes, err := json.Marshal(claim)
	if err != nil {
		return "", err
	}
	encodedClaim := encode(claimBytes)

	// create the token
	secureString := fmt.Sprintf("%s%s%s", encodedHeader, TokenSep, encodedClaim)
	signature := sign(secureString, t.secret)
	token := fmt.Sprintf("%s%s%s", secureString, TokenSep, signature)

	if len(token) > 1024 {
		return "", ErrTokenTooLong
	}
	return token, nil
}

func encodedHeader() (string, error) {
	headers := struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}{
		Algorithm: "HS256",
		Type:      "JWT",
	}

	headerBytes, err := json.Marshal(headers)
	if err != nil {
		return "", err
	}
	return encode(headerBytes), nil
}

func validate(data Data, isAdmind bool) error {
	uid, containsID := data["uid"]
	if !containsID && !isAdmind {
		return ErrNoUIDKey
	}

	if _, isString := uid.(string); containsID && !isString {
		return ErrUIDNotString
	}

	if containsID && len(uid.(string)) > 256 {
		return ErrUIDTooLong
	}
	return nil
}

func encode(data []byte) string {
	return strings.Replace(base64.URLEncoding.EncodeToString(data), "=", "", -1)
}

func sign(message, secret string) string {
	return encode(hmac.New(sha256.New, []byte(secret)).Sum([]byte(message)))
}
