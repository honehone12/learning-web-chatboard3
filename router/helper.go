package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"learning-web-chatboard3/common"
	rabbitrpc "learning-web-chatboard3/rabbit-rpc"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	runeSource         = "aA1bB2cC3dD4eE5fFgGhHiIjJkKlLm0MnNoOpPqQrRsStTuUvV6wW7xX8yY9zZ"
	macSalt            = "uPUqL7dZ"
	pwSalt             = "LV2vP8vq"
	loginCookieLabel   = "short-time"
	sessionCookieLabel = "long-time"
)

const (
	aes256KeySize uint          = 32
	macKeySize    uint          = 32
	stateSize     uint          = 32
	sessionExp    time.Duration = time.Hour * 8
	stateExp      time.Duration = time.Minute * 20
	visitExp      time.Duration = time.Hour * 24 * 365
)

var helper struct {
	block  cipher.Block
	macKey []byte
}

// every time server is restarted, cookie become no longer valid
func startHelper() (err error) {
	bKyeStr, err := generateString(aes256KeySize)
	if err != nil {
		return
	}
	bKey := []byte(bKyeStr)
	macKeyStr, err := generateString(macKeySize)
	if err != nil {
		return
	}
	helper.macKey = []byte(macKeyStr)
	helper.block, err = aes.NewCipher(bKey)
	return
}

func makeHash(plainText string) (hashed string) {
	asBytes := sha256.Sum256([]byte(plainText))
	hashed = fmt.Sprintf("%x", asBytes)
	return
}

func processPassword(pw string) string {
	// see these pkgs
	// https://pkg.go.dev/golang.org/x/crypto/bcrypt
	// https://pkg.go.dev/golang.org/x/crypto/scrypt
	return makeHash(fmt.Sprint(pwSalt, pw))
}

func generateString(length uint) (str string, err error) {
	var i uint
	maxEx := int64(len(runeSource))
	runePool := []rune(runeSource)
	for i = 0; i < length; i++ {
		bigN, err := rand.Int(rand.Reader, big.NewInt(maxEx))
		if err != nil {
			break
		}
		n := bigN.Uint64()
		str = fmt.Sprint(str, string(runePool[n]))
	}
	return
}

func encrypt(plainText string) (cipherText []byte, err error) {
	cipherText = make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]
	n, err := io.ReadFull(rand.Reader, iv)
	if err != nil {
		err = fmt.Errorf("%s: returned %d", err.Error(), n)
		return
	}

	encryptStream := cipher.NewCTR(helper.block, iv)
	encryptStream.XORKeyStream(cipherText[aes.BlockSize:], []byte(plainText))
	return
}

func decrypt(cipherText []byte) (plainText string, err error) {
	decryptText := make([]byte, len(cipherText[aes.BlockSize:]))
	decryptStream := cipher.NewCTR(helper.block, cipherText[:aes.BlockSize])
	decryptStream.XORKeyStream(decryptText, cipherText[aes.BlockSize:])
	plainText = string(decryptText)
	return
}

func makeMAC(value []byte) []byte {
	hash := hmac.New(sha256.New, helper.macKey)
	hash.Write(value)
	return hash.Sum([]byte(macSalt))
}

func verifyMAC(mac []byte, value []byte) bool {
	hash := hmac.New(sha256.New, helper.macKey)
	hash.Write(value)
	hashedVal := hash.Sum([]byte(macSalt))
	return hmac.Equal(mac, hashedVal)
}

func encode(value []byte) string {
	return base64.URLEncoding.EncodeToString(value)
}

func decode(encoded string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(encoded)
}

func sendRequest(
	client *rabbitrpc.RabbitClient,
	functionToCall string,
	dataTypeName string,
	dataPtr interface{},
	callback func(raws rabbitrpc.Raws),
) error {
	corrId := client.GenerateCorrelationID()
	callbackPool[corrId] = func(raws rabbitrpc.Raws) {
		callback(raws)
		doneCh <- raws.CorrelationId
	}
	bin, err := rabbitrpc.MakeBin(
		0,
		0,
		functionToCall,
		dataTypeName,
		dataPtr,
	)
	if err != nil {
		return err
	}

	client.Publisher.Ch <- rabbitrpc.Raws{
		Body:          bin,
		CorrelationId: corrId,
	}
	return nil
}

func sendRequestAndWait(
	client *rabbitrpc.RabbitClient,
	functionToCall string,
	dataTypeName string,
	dataPtr interface{},
	callback func(raws rabbitrpc.Raws) error,
) error {
	wait := make(chan error)
	go func() {
		err := sendRequest(
			client,
			functionToCall,
			dataTypeName,
			dataPtr,
			func(raws rabbitrpc.Raws) {
				e := callback(raws)
				wait <- e
			},
		)
		if err != nil {
			wait <- err
		}
	}()
	select {
	case e := <-wait:
		return e
	}
}

func extract(raws *rabbitrpc.Raws, dataPtr interface{}) error {
	envelop, e := rabbitrpc.FromBin(raws.Body)
	if e != nil {
		return errors.New(e.What)
	}

	if envelop.Status == rabbitrpc.StatusError {
		common.LogWarning(logger).Println("returned status is error")
		rerr := &rabbitrpc.RabbitRPCError{}
		e = envelop.Extract(rerr)
		if e != nil {
			return errors.New(e.What)
		}
		return errors.New(rerr.What)
	}

	e = envelop.Extract(dataPtr)
	if e != nil {
		return errors.New(e.What)
	}
	return nil
}

func checkLoggedIn(ctx *gin.Context) (err error) {
	uuid, err := pickupCookie(ctx, loginCookieLabel)
	if err != nil {
		return
	}
	login := &common.Login{
		UuId: uuid,
	}

	err = sendRequestAndWait(
		usersClient,
		"readLogin",
		"Login",
		login,
		func(raws rabbitrpc.Raws) (e error) {
			loginPtr := &common.Login{}
			e = extract(&raws, loginPtr)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
				return
			}

			ctx.Set(loginPtrLabel, loginPtr)
			return
		},
	)
	return
}

func checkSession(ctx *gin.Context) (err error) {
	sess := &common.Session{}
	uuid, err := pickupCookie(ctx, sessionCookieLabel)
	if err == nil {
		sess.UuId = uuid
		ctx.Set(sessionPtrLabel, sess)
		sess, err = requesSessionExist(ctx)
	}
	if err != nil {
		if gin.IsDebugging() {
			common.LogWarning(logger).
				Printf("creating new session because [%s]\n", err.Error())
		}
		sess, err = requestSessionCreate(ctx)
		if err != nil {
			return
		}
		storeSessionCookie(ctx, sess.UuId)
	}

	ctx.Set(sessionPtrLabel, sess)
	err = nil
	return
}

func storeLoginCookie(ctx *gin.Context, value string) (err error) {
	err = storeCookie(
		ctx,
		value,
		loginCookieLabel,
		sessionExp,
		0,
	)
	return
}

func storeSessionCookie(ctx *gin.Context, value string) (err error) {
	err = storeCookie(
		ctx,
		value,
		sessionCookieLabel,
		visitExp,
		60*60*24*365,
	)
	return
}

func storeCookie(
	ctx *gin.Context,
	value string,
	cookieName string,
	sessionDuration time.Duration,
	cookieDuration int,
) (err error) {
	//add exp
	value = fmt.Sprintf(
		"%s||%d",
		value,
		time.Now().Add(sessionDuration).Unix(),
	)
	encrypted, err := encrypt(value)
	if err != nil {
		return
	}
	// add mac value first
	bytesVal := makeMAC(encrypted)
	// separated '||'
	bytesVal = append(bytesVal, []byte("||")...)
	// add encrypted value
	bytesVal = append(bytesVal, encrypted...)

	valToStore := encode(bytesVal)

	if gin.IsDebugging() {
		common.LogInfo(logger).
			Printf("stored cookie [%s] %s\n", cookieName, valToStore)
	}
	ctx.SetSameSite(http.SameSiteStrictMode)
	ctx.SetCookie(
		cookieName,
		valToStore,
		cookieDuration,
		"/",
		"",
		config.UseSecureCookie,
		config.SetHttpOnlyCookie,
	)
	return
}

func pickupCookie(ctx *gin.Context, name string) (value string, err error) {
	rawStored, err := ctx.Cookie(name)
	if err != nil {
		return
	}
	bytesVal, err := decode(rawStored)
	if err != nil {
		return
	}
	splited := bytes.SplitN(bytesVal, []byte("||"), 2)
	mac := splited[0]
	encrypted := splited[1]
	if !verifyMAC(mac, encrypted) {
		err = fmt.Errorf("invalid cookie %s", rawStored)
		return
	}
	decrypted, err := decrypt(encrypted)
	if err != nil {
		return
	}
	value, unixTimeStr, ok := strings.Cut(decrypted, "||")
	if !ok {
		err = errors.New("separator not found")
		return
	}
	unixTime, err := strconv.ParseInt(unixTimeStr, 10, 64)
	if err != nil {
		return
	}

	if unixTime < time.Now().Unix() {
		err = errors.New("session expired")
	}
	return
}

func generateLoginState(ctx *gin.Context) (stateAndMACEncoded string, err error) {
	login, err := getLoginPtrFromCTX(ctx)
	if err != nil {
		return
	}

	login.State, stateAndMACEncoded, err = generateState()
	if err != nil {
		return
	}
	err = requestLoginUpdate(login, ctx)
	if err != nil {
		return
	}
	ctx.Set(loginPtrLabel, login)
	return
}

func generateSessionState(ctx *gin.Context) (stateAndMACEncoded string, err error) {
	sess, err := getSessionPtrFromCTX(ctx)
	if err != nil {
		return
	}

	sess.State, stateAndMACEncoded, err = generateState()
	if err != nil {
		return
	}
	err = requestSessionUpdate(sess, ctx)
	if err != nil {
		return
	}
	ctx.Set(sessionPtrLabel, sess)
	return
}

func generateState() (stateRaw, stateAndMACEncoded string, err error) {
	state, err := generateString(stateSize)
	if err != nil {
		return
	}
	state = fmt.Sprintf(
		"%s||%d",
		state,
		time.Now().Add(stateExp).Unix(),
	)
	stateRaw = state

	// same proc with cookie
	stateAsBytes := []byte(state)
	bytesVal := makeMAC(stateAsBytes)
	bytesVal = append(bytesVal, []byte("||")...)
	bytesVal = append(bytesVal, stateAsBytes...)
	stateAndMACEncoded = encode(bytesVal)
	return
}

func requestSessionCreate(ctx *gin.Context) (sess *common.Session, err error) {
	sess = &common.Session{}
	err = sendRequestAndWait(
		sessionsClient,
		"createSession",
		"Session",
		sess,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, sess)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func requesSessionExist(ctx *gin.Context) (sess *common.Session, err error) {
	sess, err = getSessionPtrFromCTX(ctx)
	if err != nil {
		return
	}

	err = sendRequestAndWait(
		sessionsClient,
		"readSession",
		"Session",
		sess,
		func(raws rabbitrpc.Raws) (e error) {
			e = extract(&raws, sess)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func requestSessionUpdate(sess *common.Session, ctx *gin.Context) (err error) {
	err = sendRequest(
		sessionsClient,
		"updateSession",
		"Session",
		sess,
		func(raws rabbitrpc.Raws) {
			sessPtr := &common.Session{}
			e := extract(&raws, sessPtr)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func requestLoginUpdate(login *common.Login, ctx *gin.Context) (err error) {
	err = sendRequest(
		usersClient,
		"updateLogin",
		"Login",
		login,
		func(raws rabbitrpc.Raws) {
			loginPtr := &common.Login{}
			e := extract(&raws, loginPtr)
			if e != nil {
				handleErrorInternal(e.Error(), ctx, false)
			}
			return
		},
	)
	return
}

func checkState(exposedVal, privateVal string) (err error) {
	if strings.Compare(exposedVal, "") == 0 {
		err = errors.New("exposed value is empty")
		return
	}
	if strings.Compare(privateVal, "") == 0 {
		err = errors.New("private value is empty")
		return
	}

	bytesVal, err := decode(exposedVal)
	if err != nil {
		return
	}
	splited := bytes.SplitN(bytesVal, []byte("||"), 2)
	// mac can store any bytes,
	// this should be URL encoded until validation
	macStored := splited[0]
	stateStored := string(splited[1])

	if !verifyMAC(macStored, []byte(privateVal)) {
		err = errors.New("invalid mac")
		return
	}
	if strings.Compare(stateStored, privateVal) != 0 {
		err = errors.New("invalid state")
		return
	}
	_, unixTimeStr, ok := strings.Cut(stateStored, "||")
	if !ok {
		err = errors.New("separator not found")
		return
	}
	unixTime, err := strconv.ParseInt(unixTimeStr, 10, 64)
	if err != nil {
		return
	}
	if unixTime < time.Now().Unix() {
		err = errors.New("state expired")
	}
	return
}

func loginStateCheckProcess(ctx *gin.Context) (login *common.Login, err error) {
	login, err = getLoginPtrFromCTX(ctx)
	if err != nil {
		return
	}

	// check state
	state := ctx.PostForm("state")
	err = checkState(state, login.State)
	if err != nil {
		return
	}

	// state is consumed, delete it
	login.State = ""
	err = requestLoginUpdate(login, ctx)
	return
}

func sessionStateCheckProcess(ctx *gin.Context) (sess *common.Session, err error) {
	sess, err = getSessionPtrFromCTX(ctx)
	if err != nil {
		return
	}

	// check state
	state := ctx.PostForm("state")
	err = checkState(state, sess.State)
	if err != nil {
		return
	}

	// state is consumed, delete it
	sess.State = ""
	err = requestSessionUpdate(sess, ctx)
	return
}
