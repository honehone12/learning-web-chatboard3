package main

import (
	"errors"
	"fmt"
	"html/template"
	"learning-web-chatboard3/common"
	rabbitrpc "learning-web-chatboard3/rabbit-rpc"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	publicNavbar template.HTML = `
<div class="navbar navbar-expand-md navbar-dark fixed-top bg-dark" role="navigation">
  <div class="container">
    <div class="navbar-header">
      <a class="navbar-brand" href="/">KEIJIBAN</a>
    </div>
    <div class="nav navbar-nav navbar-right">
      <a href="/user/login">Login</a>
    </div>
  </div>
</div>`

	privateNavbar template.HTML = `
<div class="navbar navbar-expand-md navbar-dark fixed-top bg-dark" role="navigation">
  <div class="container">
    <div class="navbar-header">
	  <a class="navbar-brand" href="/">KEIJIBAN</a>
    </div>
    <div class="nav navbar-nav navbar-right">
	  <a href="/user/logout">Logout</a>
    </div>
  </div>
</div>`

	replyForm template.HTML = `
<div class="panel panel-info">
  <div class="panel-body">
    <form id="post" role="form" action="/topic/post" method="post">
	  <div class="form-group">
	  	<textarea class="form-control" name="body" id="body" placeholder="Write your reply here" rows="3"></textarea>
	    <br>
		<button class="btn btn-primary pull-right" type="submit">Reply</button>
	  </div>
    </form>
  </div>
</div>`
)

func handleErrorInternal(
	loggerErrorMsg string,
	ctx *gin.Context,
	redirect bool,
) {
	common.LogError(logger).Println(loggerErrorMsg)
	if redirect {
		errorRedirect(ctx, "internal error")
	}
}

func getHTMLElemntInternal(isLoggedin bool) (template.HTML, template.HTML) {
	if isLoggedin {
		return privateNavbar, replyForm
	} else {
		return publicNavbar, ""
	}
}

func indexGet(ctx *gin.Context) {
	topics, err := indexGetInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
	}
	navbar, _ := getHTMLElemntInternal(confirmLoggedIn(ctx))
	ctx.HTML(
		http.StatusOK,
		"index.html",
		gin.H{
			"navbar": navbar,
			"topics": topics,
		},
	)
}

func indexGetInternal(ctx *gin.Context) (topics []common.Topic, err error) {
	err = sendRequestAndWait(
		topicsClient,
		"readTopics",
		"Topic",
		&common.Topic{},
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, &topics)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func errorRedirect(ctx *gin.Context, msg string) {
	ctx.Redirect(
		http.StatusFound,
		fmt.Sprintf(
			"%s%s",
			"/error?msg=",
			msg,
		),
	)
}

func errorGet(ctx *gin.Context) {
	errMsg := ctx.Query("msg")
	err := validate.Var(errMsg, "lowercase")
	if err != nil {
		errMsg = "internal error"
	}
	navbar, _ := getHTMLElemntInternal(confirmLoggedIn(ctx))
	ctx.HTML(
		http.StatusFound,
		"error.html",
		gin.H{
			"navbar": navbar,
			"msg":    errMsg,
		},
	)
}

func loginGet(ctx *gin.Context) {
	state := getStateFromCTX(ctx)
	ctx.HTML(
		http.StatusOK,
		"login.html",
		gin.H{
			"state": state,
		},
	)
}

func signupGet(ctx *gin.Context) {
	state := getStateFromCTX(ctx)
	ctx.HTML(
		http.StatusOK,
		"signup.html",
		gin.H{
			"state": state,
		},
	)
}

func logoutGet(ctx *gin.Context) {
	if confirmLoggedIn(ctx) {
		err := logoutGetInternal(ctx)
		if err != nil {
			handleErrorInternal(err.Error(), ctx, true)
			return
		}
	}
	ctx.Redirect(http.StatusMovedPermanently, "/")
}

func logoutGetInternal(ctx *gin.Context) (err error) {
	login, err := getLoginPtrFromCTX(ctx)
	if err != nil {
		return
	}

	err = sendRequest(
		usersClient,
		"deleteLogin",
		"Login",
		login,
		func(raws rabbitrpc.Raws) {
			e := extract(&raws, &common.SimpleMessage{})
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
		},
	)
	return
}

func signupPost(ctx *gin.Context) {
	err := signupPostInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
		return
	}
	ctx.Redirect(http.StatusMovedPermanently, "/user/login")
}

func signupPostInternal(ctx *gin.Context) (err error) {
	_, err = sessionStateCheckProcess(ctx)
	if err != nil {
		return
	}

	var salt string
	salt, err = generateString(pwSaltSize)
	if err != nil {
		return
	}
	pw := processPassword(ctx.PostForm("password"), salt)
	newUser := common.User{
		Name:     ctx.PostForm("name"),
		Email:    ctx.PostForm("email"),
		Password: pw,
		Salt:     salt,
	}

	err = sendRequest(
		usersClient,
		"createUser",
		"User",
		&newUser,
		func(raws rabbitrpc.Raws) {
			user := &common.User{}
			e := extract(&raws, user)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
		},
	)
	return
}

func authenticatePost(ctx *gin.Context) {
	err := authenticatePostInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
		return
	}
	ctx.Redirect(http.StatusMovedPermanently, "/")
}

func authenticatePostInternal(ctx *gin.Context) (err error) {
	_, err = sessionStateCheckProcess(ctx)
	if err != nil {
		return
	}

	email := ctx.PostForm("email")
	err = validate.Var(email, "email")
	if err != nil {
		return
	}

	authUser := common.User{
		Email: email,
	}
	err = sendRequestAndWait(
		usersClient,
		"readUser",
		"User",
		&authUser,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, &authUser)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	if err != nil {
		return
	}

	pw := processPassword(ctx.PostForm("password"), authUser.Salt)
	if strings.Compare(authUser.Password, pw) != 0 {
		err = errors.New("password mismatch")
		return
	}

	// delete invalid login data in db first
	delSess := common.Login{
		UserName: authUser.Name,
		UserId:   authUser.Id,
	}
	err = sendRequest(
		usersClient,
		"deleteLogin",
		"Login",
		&delSess,
		func(raws rabbitrpc.Raws) {
			e := extract(&raws, &common.SimpleMessage{})
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
		},
	)

	// start new session
	login := common.Login{}
	err = sendRequestAndWait(
		usersClient,
		"createLogin",
		"User",
		&authUser,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, &login)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	if err != nil {
		return
	}

	// actual login starts here
	err = storeLoginCookie(ctx, login.UuId)
	return
}

func topicGet(ctx *gin.Context) {
	topic, replies, err := topicGetInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
		return
	}

	navbar, replyForm := getHTMLElemntInternal(confirmLoggedIn(ctx))
	state := getStateFromCTX(ctx)

	ctx.HTML(
		http.StatusOK,
		"topic.html",
		gin.H{
			"navbar":    navbar,
			"topic":     topic,
			"replyForm": replyForm,
			"replies":   replies,
			"state":     state,
		},
	)
}

func topicGetInternal(ctx *gin.Context,
) (topic *common.Topic, replies []common.Reply, err error) {
	base64_uuid := ctx.Query("id")
	bytes, err := decode(base64_uuid)
	if err != nil {
		return
	}

	uuid := string(bytes)
	err = validate.Var(uuid, "uuid4")
	if err != nil {
		return
	}

	topic = &common.Topic{UuId: uuid}
	err = sendRequestAndWait(
		topicsClient,
		"readATopic",
		"Topic",
		topic,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, topic)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	if err != nil {
		return
	}

	err = sendRequestAndWait(
		topicsClient,
		"readRepliesInTopic",
		"Topic",
		topic,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, &replies)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	if err != nil {
		return
	}

	// store info into session
	sess, err := getSessionPtrFromCTX(ctx)
	if err != nil {
		return
	}
	sess.TopicId = topic.Id
	sess.TopicUuId = topic.UuId
	err = requestSessionUpdate(sess, ctx)
	return
}

func newTopicGet(ctx *gin.Context) {
	loggedin := confirmLoggedIn(ctx)
	navbar, _ := getHTMLElemntInternal(loggedin)
	state := getStateFromCTX(ctx)
	if loggedin {
		ctx.HTML(
			http.StatusOK,
			"newtopic.html",
			gin.H{
				"navbar": navbar,
				"state":  state,
			},
		)
	} else {
		ctx.Redirect(http.StatusFound, "/user/login")
	}
}

func newTopicPost(ctx *gin.Context) {
	if !confirmLoggedIn(ctx) {
		ctx.Redirect(http.StatusFound, "/user/login")
		return
	}

	err := newTopicPostInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
		return
	}

	ctx.Redirect(http.StatusMovedPermanently, "/")
}

func newTopicPostInternal(ctx *gin.Context) (err error) {
	login, err := loginStateCheckProcess(ctx)
	if err != nil {
		return
	}

	topic := common.Topic{
		Topic:  ctx.PostForm("topic"),
		Owner:  login.UserName,
		UserId: login.UserId,
	}
	err = sendRequestAndWait(
		topicsClient,
		"createTopic",
		"Topic",
		&topic,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, &topic)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func newReplyPost(ctx *gin.Context) {
	if !confirmLoggedIn(ctx) {
		ctx.Redirect(http.StatusFound, "/user/login")
		return
	}

	topiUuId, err := newReplyPostInternal(ctx)
	if err != nil {
		handleErrorInternal(err.Error(), ctx, true)
		return
	}
	encoded := encode([]byte(topiUuId))
	ctx.Redirect(http.StatusMovedPermanently, fmt.Sprint("/topic/read?id=", encoded))
}

func newReplyPostInternal(ctx *gin.Context) (topiUuId string, err error) {
	login, err := loginStateCheckProcess(ctx)
	if err != nil {
		return
	}

	// pick up info from session
	sess, err := getSessionPtrFromCTX(ctx)
	topiId := sess.TopicId
	topiUuId = sess.TopicUuId

	body := ctx.PostForm("body")

	reply := common.Reply{
		Body:        body,
		Contributor: login.UserName,
		UserId:      login.UserId,
		TopicId:     topiId,
	}
	err = sendRequest(
		topicsClient,
		"createReply",
		"Reply",
		&reply,
		func(raws rabbitrpc.Raws) {
			e := extract(&raws, &reply)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
		},
	)
	if err != nil {
		return
	}

	topic := common.Topic{UuId: topiUuId}
	err = sendRequest(
		topicsClient,
		"incrementTopic",
		"Topic",
		&topic,
		func(raws rabbitrpc.Raws) {
			e := extract(&raws, &topic)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
		},
	)
	return
}
