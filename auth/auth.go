package auth

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/swishcloud/gostudy/common"
	"github.com/swishcloud/gostudy/keygenerator"

	"github.com/golang-jwt/jwt/v5"
	"github.com/swishcloud/goweb"
	"golang.org/x/oauth2"
)

var access_token_cookie_name string

const csrf_state_cookie_name = "crft_state"
const pkce_cookie_name = "pkce"

var sessions []session

type session struct {
	id     string
	token  *oauth2.Token
	Claims map[string]interface{}
	Data   map[string]interface{}
}

func (s *session) GetAccessToken(conf *oauth2.Config) (string, error) {
	if token, err := s.getToken((conf)); err != nil {
		return "", err
	} else {
		return token.AccessToken, nil
	}
}
func (s *session) getToken(conf *oauth2.Config) (*oauth2.Token, error) {
	ts := conf.TokenSource(context.Background(), s.token)
	new_token, err := ts.Token()
	if err != nil {
		return nil, err
	}
	if new_token.AccessToken != s.token.AccessToken {
		s.token = new_token
		log.Println("refreshed a token")
	}
	return s.token, nil
}
func Login(ctx *goweb.Context, token *oauth2.Token, jwk_json_url string, expire_time *time.Time) *session {
	//todo:mutex.Lock()
	//todo:defer mutex.Unlock()
	session := session{}
	session.id = uuid.New().String()
	session.token = token
	session.Claims = extractIdTokenClaims(token.Extra("id_token").(string), jwk_json_url)
	session.Data = map[string]interface{}{}
	var cookie http.Cookie
	if expire_time == nil {
		cookie = http.Cookie{Name: access_token_cookie_name, Value: session.id, Path: "/", HttpOnly: true, Secure: true}
	} else {
		cookie = http.Cookie{Name: access_token_cookie_name, Value: session.id, Path: "/", HttpOnly: true, Secure: true, Expires: *expire_time}
	}
	sessions = append(sessions, session)
	http.SetCookie(ctx.Writer, &cookie)
	return &session
}
func Logout(rac *common.RestApiClient, ctx *goweb.Context, conf *oauth2.Config, introspectTokenURL string, skip_tls_verify bool, postLogout func(id_token string)) {
	common.DelCookie(ctx.Writer, access_token_cookie_name)
	s, err := GetSessionByToken(rac, ctx, conf, introspectTokenURL, skip_tls_verify)
	if err != nil {
		panic(err)
	}
	postLogout(s.token.Extra("id_token").(string))
}

func HasLoggedIn(rac *common.RestApiClient, ctx *goweb.Context, conf *oauth2.Config, introspectTokenURL string, skip_tls_verify bool) bool {
	_, err := GetSessionByToken(rac, ctx, conf, introspectTokenURL, skip_tls_verify)
	return err == nil
}
func CheckToken(rac *common.RestApiClient, token *oauth2.Token, introspectTokenURL string, skip_tls_verify bool) (ok bool, sub string, err error) {
	rar := common.NewRestApiRequest("GET", introspectTokenURL, nil).SetAuthHeader(token)
	resp, err := rac.Do(rar)
	if err != nil {
		return false, "", err
	}
	m, err := common.ReadAsMap(resp.Body)
	if err != nil {
		return false, "", err
	}
	if m["error"] != nil {
		return false, "", errors.New(m["error"].(string))
	}
	data := m["data"].(map[string]interface{})
	isActive := data["active"].(bool)
	sub = ""
	if isActive {
		sub = data["sub"].(string)
	}
	return isActive, sub, nil
}
func GetBearerToken(ctx *goweb.Context) (string, error) {
	authorization := ctx.Request.Header["Authorization"]
	if len(authorization) == 0 {
		return "", errors.New("not found bearer token")
	}
	if match, _ := regexp.MatchString("Bearer .+", authorization[0]); !match {
		return "", errors.New("not found bearer token")
	}
	token := []rune(authorization[0])
	token = token[7:]
	return string(token), nil
}
func removeSessionAt(index int) {
	sessions = append(sessions[:index], sessions[index+1:]...)
}
func GetSessionByToken(rac *common.RestApiClient, ctx *goweb.Context, conf *oauth2.Config, introspectTokenURL string, skip_tls_verify bool) (*session, error) {
	cookie, err := ctx.Request.Cookie(access_token_cookie_name)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(sessions); i++ {
		s := &sessions[i]
		if s.id == cookie.Value {
			token, err := s.getToken(conf)
			if err != nil {
				return nil, err
			}
			ok, _, err := CheckToken(rac, token, introspectTokenURL, skip_tls_verify)
			if err != nil {
				return nil, err
			}
			if !ok {
				removeSessionAt(i)
				return nil, errors.New("the login session has expired.")
			}
			return s, nil
		}
	}
	return nil, errors.New("not found session")
}
func init() {
	k, err := keygenerator.NewKey(4, false, false, false, true)
	if err != nil {
		panic(err)
	}
	access_token_cookie_name = "access_token_" + k
}

func extractIdTokenClaims(tokenString string, jwkJSONURL string) map[string]interface{} {
	keySet, err := jwk.Fetch(jwkJSONURL)
	if err != nil {
		panic(err)
	}

	if len(keySet.Keys) == 0 {
		panic("JWK set contains no keys")
	}

	k, err := keySet.Keys[0].Materialize()
	if err != nil {
		panic(err)
	}

	pk, ok := k.(*rsa.PublicKey)
	if !ok {
		panic(fmt.Errorf("expected RSA public key, got %T", k))
	}

	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			// Verify that the signing method is RSA.
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					token.Header["alg"],
				)
			}

			return pk, nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodRS384.Alg(),
			jwt.SigningMethodRS512.Alg(),
		}),
	)

	if err != nil {
		panic(err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return map[string]interface{}(claims)
	}

	return nil
}
func AuthCodeURL(ctx *goweb.Context, conf *oauth2.Config) (string, error) {
	//state
	state, err := keygenerator.NewKey(20, false, false, false, false)
	if err != nil {
		return "", err
	}
	state = base64.URLEncoding.EncodeToString([]byte(state))
	cookie := http.Cookie{Name: csrf_state_cookie_name, Value: state, Path: "/", Secure: true, HttpOnly: true}
	http.SetCookie(ctx.Writer, &cookie)
	if err != nil {
		return "", err
	}
	//pcke
	pkce, err := keygenerator.NewKey(43, false, false, false, true)
	if err != nil {
		return "", err
	}
	sha256_hased_pkce := sha256.Sum256([]byte(pkce))
	encoded_pkce := base64.StdEncoding.EncodeToString(sha256_hased_pkce[:])
	encoded_pkce = strings.Replace(encoded_pkce, "=", "", -1)
	encoded_pkce = strings.Replace(encoded_pkce, "+", "-", -1)
	encoded_pkce = strings.Replace(encoded_pkce, "/", "_", -1)
	pkce_cookie := http.Cookie{Name: pkce_cookie_name, Value: pkce, Path: "/", Secure: true, HttpOnly: true}
	http.SetCookie(ctx.Writer, &pkce_cookie)
	if err != nil {
		return "", err
	}
	return conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("code_challenge", encoded_pkce), oauth2.SetAuthURLParam("code_challenge_method", "S256")), nil
}
func Exchange(ctx *goweb.Context, conf *oauth2.Config, http_client *http.Client) (*oauth2.Token, error) {
	//state
	state := ctx.Request.URL.Query().Get("state")
	common.DelCookie(ctx.Writer, csrf_state_cookie_name)
	if cookie, err := ctx.Request.Cookie(csrf_state_cookie_name); err != nil {
		return nil, errors.New("state cookie does not present")
	} else {
		if cookie.Value != state {
			return nil, errors.New("csrf verification failed")
		}
	}
	//pkce
	pkce := ""
	common.DelCookie(ctx.Writer, pkce_cookie_name)
	if cookie, err := ctx.Request.Cookie(pkce_cookie_name); err != nil {
		return nil, errors.New("pkce cookie does not present")
	} else {
		pkce = cookie.Value
	}
	code := ctx.Request.URL.Query().Get("code")
	token, err := conf.Exchange(context.WithValue(context.Background(), "", http_client), code, oauth2.SetAuthURLParam("code_verifier", pkce))
	if err != nil {
		return nil, err
	}
	return token, err
}
