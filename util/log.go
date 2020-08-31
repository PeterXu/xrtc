package util

import (
	"log"
	"os"
)

type _LogBase interface {
	Print(v ...interface{})
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type _LogExtend interface {
	Warn(v ...interface{})
	Warnln(v ...interface{})
	Warnf(format string, v ...interface{})
	Error(v ...interface{})
	Errorln(v ...interface{})
	Errorf(format string, v ...interface{})
}

type _LogExtendImpl struct {
	_LogBase
}

func (x *_LogExtendImpl) Warn(v ...interface{}) {
	x.Print(v...)
}
func (x *_LogExtendImpl) Warnln(v ...interface{}) {
	x.Println(v...)
}
func (x *_LogExtendImpl) Warnf(format string, v ...interface{}) {
	x.Printf(format, v...)
}
func (x *_LogExtendImpl) Error(v ...interface{}) {
	x.Print(v...)
}
func (x *_LogExtendImpl) Errorln(v ...interface{}) {
	x.Println(v...)
}
func (x *_LogExtendImpl) Errorf(format string, v ...interface{}) {
	x.Printf(format, v...)
}

/// Internal log struct

type _Logger2 struct {
	_LogBase
	_LogExtend
}

var app_log _Logger2

func init() {
	// default use golang::log
	goLog := log.New(os.Stderr, "", log.LstdFlags)
	app_log._LogBase = goLog
	app_log._LogExtend = &_LogExtendImpl{goLog}
}

/// External log interface

type LogObject interface {
	_LogBase
	_LogExtend
}

func SetLogObject(obj LogObject) {
	if obj != nil {
		app_log._LogBase = obj
		app_log._LogExtend = obj
	}
}

/// External log functions

func LogPrint(v ...interface{}) {
	app_log.Print(v...)
}

func LogPrintln(v ...interface{}) {
	app_log.Println(v...)
}

func LogPrintf(format string, v ...interface{}) {
	app_log.Printf(format, v...)
}

func LogWarn(v ...interface{}) {
	app_log.Warn(v...)
}

func LogWarnln(v ...interface{}) {
	app_log.Warnln(v...)
}

func LogWarnf(format string, v ...interface{}) {
	app_log.Warnf(format, v...)
}

func LogError(v ...interface{}) {
	app_log.Error(v...)
}

func LogErrorln(v ...interface{}) {
	app_log.Errorln(v...)
}

func LogErrorf(format string, v ...interface{}) {
	app_log.Errorf(format, v...)
}
