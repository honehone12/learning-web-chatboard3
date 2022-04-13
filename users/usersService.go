package main

import (
	"errors"
	"fmt"
	"learning-web-chatboard3/common"
	"time"
)

const (
	usersTable  = "users"
	loginsTable = "logins"
)

func createUser(user *common.User, corrId string) {
	err := createUserInternal(user)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, user, "User", corrId)
}

func createUserInternal(user *common.User) (err error) {
	if common.IsEmpty(
		user.Name,
		user.Email,
		user.Password,
	) {
		err = errors.New("contains empty string")
		return
	}
	user.UuId = common.NewUuIdString()
	user.CreatedAt = time.Now()
	err = createUserSQL(user)
	return
}

func createUserSQL(user *common.User) (err error) {
	affected, err := dbEngine.
		Table(usersTable).
		InsertOne(user)
	if err == nil && affected != 1 {
		err = fmt.Errorf(
			"something wrong. returned value was %d",
			affected,
		)
	}
	return
}

func createLogin(user *common.User, corrId string) {
	login, err := createLoginInternal(user)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, login, "Login", corrId)
}

func createLoginInternal(user *common.User) (login *common.Login, err error) {
	if common.IsEmpty(user.Name, user.Email) {
		err = errors.New("contains empty string")
		return
	}
	now := time.Now()
	login = &common.Login{
		UuId:       common.NewUuIdString(),
		UserName:   user.Name,
		UserId:     user.Id,
		LastUpdate: now,
		CreatedAt:  now,
	}
	err = createLoginSQL(login)
	return
}

func createLoginSQL(login *common.Login) (err error) {
	affected, err := dbEngine.
		Table(loginsTable).
		InsertOne(login)
	if err == nil && affected != 1 {
		err = fmt.Errorf(
			"something's wrong. returned value was %d",
			affected,
		)
	}
	return
}

func readUser(user *common.User, corrId string) {
	err := readUserInternal(user)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, user, "User", corrId)
}

func readUserInternal(user *common.User) (err error) {
	if common.IsEmpty(user.Email) && common.IsEmpty(user.UuId) {
		err = errors.New("need email or uuid for finding user")
		return
	}
	err = readUserSQL(user)
	return
}

func readUserSQL(user *common.User) (err error) {
	var ok bool
	ok, err = dbEngine.
		Table(usersTable).
		Get(user)
	if err == nil && !ok {
		err = errors.New("no such user")
	}
	return
}

func readLogin(login *common.Login, corrId string) {
	err := readLoginInternal(login)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, login, "Login", corrId)
}

func readLoginInternal(login *common.Login) (err error) {
	if common.IsEmpty(login.UuId) {
		err = errors.New("need uuid for finding login")
		return
	}
	err = readLoginSQL(login)
	return
}

func readLoginSQL(login *common.Login) (err error) {
	var ok bool
	ok, err = dbEngine.
		Table(loginsTable).
		Get(login)
	if err == nil && !ok {
		err = errors.New("no such login")
	}
	return
}

func updateLogin(login *common.Login, corrId string) {
	err := updateLoginInternal(login)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
		return
	}

	common.SendOK(server, login, "Login", corrId)
}

func updateLoginInternal(login *common.Login) (err error) {
	if common.IsEmpty(
		login.UuId,
		login.UserName,
	) {
		err = fmt.Errorf("contains empty string %s %s", login.UuId, login.UserName)
		return
	}
	login.LastUpdate = time.Now()
	err = updateLoginSQL(login)
	return
}

func updateLoginSQL(login *common.Login) (err error) {
	affected, err := dbEngine.
		Table(loginsTable).
		ID(login.Id).
		Update(login)
	if err == nil && affected != 1 {
		err = fmt.Errorf(
			"something wrong. returned value was %d",
			affected,
		)
	}
	return
}

func deleteLogin(login *common.Login, corrId string) {
	err := deleteLoginInternal(login)
	if err != nil {
		common.HandleError(server, logger, err.Error(), corrId)
	}

	msg := &common.SimpleMessage{
		Message: "deleted",
	}
	common.SendOK(server, msg, "SimpleMessage", corrId)
}

func deleteLoginInternal(login *common.Login) (err error) {
	err = deleteLoginSQL(login)
	return
}

func deleteLoginSQL(login *common.Login) (err error) {
	affected, err := dbEngine.
		Table(loginsTable).
		Delete(login)

	common.LogInfo(logger).Printf("deleted %d linked %v", affected, *login)
	return
}
