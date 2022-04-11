package main

import (
	"errors"
	"fmt"
	"learning-web-chatboard3/common"
	"time"
)

const sessionsTable = "sessions"

func createSession(corrId string) {
	var sess common.Session
	err := createSessionInternal(&sess)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, &sess, "Session", corrId)
}

func createSessionInternal(sess *common.Session) (err error) {
	now := time.Now()
	sess.UuId = common.NewUuIdString()
	sess.CreatedAt = now

	err = createSessionSQL(sess)
	return
}

func readSession(sess *common.Session, corrId string) {
	err := readSessionInternal(sess)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, sess, "Session", corrId)
}

func readSessionInternal(sess *common.Session) (err error) {
	if common.IsEmpty(sess.UuId) {
		err = errors.New("need uuid for finding session")
		return
	}
	err = readSessionSQL(sess)
	return
}

func updateSession(sess *common.Session, corrId string) {
	err := updateSessionInternal(sess)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, sess, "Session", corrId)
}

func updateSessionInternal(sess *common.Session) (err error) {
	if common.IsEmpty(
		sess.UuId,
	) {
		err = fmt.Errorf("contains empty string %s %s", sess.UuId, sess.State)
		return
	}
	err = updateSessionSQL(sess)
	return
}

func createSessionSQL(sess *common.Session) (err error) {
	affected, err := dbEngine.
		Table(sessionsTable).
		InsertOne(sess)
	if err == nil && affected != 1 {
		err = fmt.Errorf(
			"something wrong. returned value was %d",
			affected,
		)
	}
	return
}

func readSessionSQL(sess *common.Session) (err error) {
	var ok bool
	ok, err = dbEngine.
		Table(sessionsTable).
		Get(sess)
	if err == nil && !ok {
		err = errors.New("no such session")
	}
	return
}

func updateSessionSQL(sess *common.Session) (err error) {
	affected, err := dbEngine.
		Table(sessionsTable).
		ID(sess.Id).
		Update(sess)
	if err == nil && affected != 1 {
		err = fmt.Errorf(
			"something wrong. returned value was %d",
			affected,
		)
	}
	return
}
