package main

import (
	"learning-web-chatboard3/common"
	rabbitrpc "learning-web-chatboard3/rabbit-rpc"
	"log"

	"xorm.io/xorm"
)

var dbEngine *xorm.Engine
var config *common.Configuration
var logger *log.Logger
var server *rabbitrpc.RabbitClient

func main() {
	var err error

	// config
	config, err = common.LoadConfig()
	if err != nil {
		log.Fatalln(err.Error())
	}

	//log
	logger, err = common.OpenLogger(
		config.LogToFile,
		config.LogFileNameUsers,
	)
	if err != nil {
		log.Fatal(err.Error())
	}

	//database
	dbEngine, err = common.OpenDb(
		config.DbName,
		config.ShowSQL,
		0,
	)
	if err != nil {
		common.LogError(logger).Fatalln(err.Error())
	}

	//rabbit
	server = rabbitrpc.NewRPCServer(
		rabbitrpc.DefaultRabbitURL,
		config.SessionsResQName,
		config.SessionsReqQName,
		config.SessionsExchangeName,
		rabbitrpc.ExchangeKindDirect,
		config.SessionsClientKey,
		config.SessionsServerKey,
		onRequestReceived,
	)
	defer server.Publisher.Done()
	defer server.Subscriber.Done()

	select {
	case <-server.Publisher.CTX.Done():
		break
	case <-server.Subscriber.CTX.Done():
		break
	}
}

func onRequestReceived(raws rabbitrpc.Raws) {
	go func() {
		var err *rabbitrpc.RabbitRPCError
		envelop, err := rabbitrpc.FromBin(raws.Body)
		if err != nil {
			common.SendError(server, err, raws.CorrelationId)
			return
		}

		err = routingRequest(envelop, raws.CorrelationId)
		if err != nil {
			common.SendError(server, err, raws.CorrelationId)
		}
	}()
}

func routingRequest(envelop *rabbitrpc.Envelope, corrId string,
) (err *rabbitrpc.RabbitRPCError) {
	switch envelop.DataTypeName {
	case "Session":
		var sess common.Session
		err = envelop.Extract(&sess)
		if err != nil {
			return
		}

		switch envelop.FunctionToCall {
		case "createSession":
			createSession(corrId)
		case "readSession":
			readSession(&sess, corrId)
		case "updateSession":
			updateSession(&sess, corrId)
		default:
			err = rabbitrpc.ErrorFunctionNotFound
		}

	default:
		err = rabbitrpc.ErrorTypeNotFound
	}
	return
}
