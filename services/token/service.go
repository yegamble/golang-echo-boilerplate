package token

import (
	"echo-demo-project/models"
	s "echo-demo-project/server"
	"encoding/json"
	"fmt"
	"time"

	jwtGo "github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const ExpireAccessMinutes = 30
const ExpireRefreshMinutes = 2 * 60

type JwtCustomClaims struct {
	ID  uint   `json:"id"`
	UID string `json:"uid"`
	jwtGo.StandardClaims
}

type CachedTokens struct {
	AccessUID  string `json:"access"`
	RefreshUID string `json:"refresh"`
}

type ServiceWrapper interface {
	GenerateTokenPair(user *models.User) (accessToken, refreshToken string, exp int64, err error)
	ParseToken(tokenString, secret string) (claims *jwtGo.MapClaims, err error)
}

type Service struct {
	server *s.Server
}

func NewTokenService(server *s.Server) *Service {
	return &Service{
		server: server,
	}
}

func (tokenService *Service) GenerateTokenPair(user *models.User) (
	accessToken string,
	refreshToken string,
	exp int64,
	err error,
) {
	var accessUID, refreshUID string
	if accessToken, accessUID, exp, err = tokenService.createToken(user.ID, ExpireAccessMinutes,
		tokenService.server.Config.Auth.AccessSecret); err != nil {
		return
	}

	if refreshToken, refreshUID, _, err = tokenService.createToken(user.ID, ExpireRefreshMinutes,
		tokenService.server.Config.Auth.RefreshSecret); err != nil {
		return
	}

	cacheJSON, err := json.Marshal(CachedTokens{
		AccessUID:  accessUID,
		RefreshUID: refreshUID,
	})
	tokenService.server.Redis.Set(fmt.Sprintf("token-%d", user.ID), string(cacheJSON), 0)

	return
}

func (tokenService *Service) ParseToken(tokenString, secret string) (claims *jwtGo.MapClaims, err error) {
	token, err := jwtGo.Parse(tokenString, func(token *jwtGo.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwtGo.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return
	}

	if claims, ok := token.Claims.(jwtGo.MapClaims); ok && token.Valid {
		return &claims, nil
	}

	return nil, err
}

func (tokenService *Service) createToken(userID uint, expireMinutes int, secret string) (
	token string,
	uid string,
	exp int64,
	err error,
) {
	exp = time.Now().Add(time.Minute * time.Duration(expireMinutes)).Unix()
	uid = uuid.New().String()
	claims := &JwtCustomClaims{
		ID:  userID,
		UID: uid,
		StandardClaims: jwtGo.StandardClaims{
			ExpiresAt: exp,
		},
	}
	jwtToken := jwtGo.NewWithClaims(jwtGo.SigningMethodHS256, claims)
	token, err = jwtToken.SignedString([]byte(secret))

	return
}
