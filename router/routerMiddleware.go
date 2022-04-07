package main

import (
	"errors"
	"learning-web-chatboard3/common"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	loggedInLabel   = "logged-in"
	loginPtrLabel   = "login-ptr"
	sessionPtrLabel = "session-ptr"
	stateLabel      = "state"
)

func SessionCheckMiddleware(ctx *gin.Context) {
	err := checkSession(ctx)
	if err != nil {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln(err.Error())
		} else {
			common.LogError(logger).Printf("!!MIDDLEWARE ERROR!! %s\n", err.Error())
			return
		}
	}
	ctx.Header("Cache-Control", "no-store")
	ctx.Next()
}

func LoggedInCheckMiddleware(ctx *gin.Context) {
	err := checkLoggedIn(ctx)
	if err != nil {
		common.LogWarning(logger).Println(err.Error())
	}
	ctx.Set(loggedInLabel, err == nil)
	ctx.Header("Cache-Control", "no-store")
	ctx.Next()
}

func GenerateLoginStateMiddleware(ctx *gin.Context) {
	if !confirmLoggedIn(ctx) {
		return
	}

	state, err := generateLoginState(ctx)
	if err != nil {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln(err.Error())
		} else {
			common.LogError(logger).Printf("!!MIDDLEWARE NOTWORKING!! %s\n", err.Error())
		}
	}
	ctx.Set(stateLabel, state)
	ctx.Next()
}

func GenerateSessionStateMiddleware(ctx *gin.Context) {
	state, err := generateSessionState(ctx)
	if err != nil {
		// safety for invalid cookie
		if strings.Compare(err.Error(), "invalid cookie") == 0 {
			ctx.Redirect(http.StatusFound, "/")
			return
		}

		if gin.IsDebugging() {
			common.LogError(logger).Fatalln(err.Error())
		} else {
			common.LogError(logger).Printf("!!MIDDLEWARE NOTWORKING!! %s\n", err.Error())
		}
	}

	ctx.Set(stateLabel, state)
	ctx.Next()
}

// belowes are related utils ///////////////////////////////////////

func confirmLoggedIn(ctx *gin.Context) (isLoggedIn bool) {
	loggedInVal, ok := ctx.Get(loggedInLabel)
	if !ok {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln("logged-in not stored")
		} else {
			common.LogError(logger).Println("!!MIDDLEWARE NOT WORKING!! logged-in not stored")
		}
		return
	}
	isLoggedIn, ok = loggedInVal.(bool)
	if !ok {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln("logged-in is not boolean")
		} else {
			common.LogError(logger).Println("!!MIDDLEWARE BROKEN!! logged-in is not boolean")
		}
	}
	return
}

func getLoginPtrFromCTX(ctx *gin.Context) (ptr *common.Login, err error) {
	val, ok := ctx.Get(loginPtrLabel)
	if !ok {
		err = errors.New("not logged in")
		return
	}
	if ptr, ok = val.(*common.Login); !ok {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln("login-ptr is not *Login")
		}
		err = errors.New("!!MIDDLEWARE BROKEN!! login-ptr is not *Login")
	}
	return
}

func getSessionPtrFromCTX(ctx *gin.Context) (ptr *common.Session, err error) {
	val, ok := ctx.Get(sessionPtrLabel)
	if !ok {
		err = errors.New("session-ptr is not stored")
		return
	}
	if ptr, ok = val.(*common.Session); !ok {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln("session-ptr is not *Session")
		}
		err = errors.New("!!MIDDLEWARE BROKEN!! session-ptr is not *Session")
	}
	return
}

func getStateFromCTX(ctx *gin.Context) (state string) {
	val, ok := ctx.Get(stateLabel)
	if !ok {
		common.LogWarning(logger).Println("state not generated yet")
		return
	}
	if state, ok = val.(string); !ok {
		if gin.IsDebugging() {
			common.LogError(logger).Fatalln("state is not string")
		} else {
			common.LogError(logger).Println("!!MIDDLEWARE BROKEN!! state is not string")
		}
	}
	return
}
