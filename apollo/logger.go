package apollo

import (
	"github.com/apolloconfig/agollo/v4/component/log"
	"github.com/cloudwego/kitex/pkg/klog"
)

type customApolloLogger struct{}

func NewCustomApolloLogger() log.LoggerInterface {
	return customApolloLogger{}
}

func (m customApolloLogger) Info(v ...interface{}) {
	klog.Info(v...)
}

func (m customApolloLogger) Warn(v ...interface{}) {
	klog.Warn(v...)
}

func (m customApolloLogger) Error(v ...interface{}) {
	klog.Error(v...)
}

func (m customApolloLogger) Debug(v ...interface{}) {
	klog.Debug(v)
}

func (m customApolloLogger) Infof(fmt string, v ...interface{}) {
	klog.Infof(fmt, v...)
}

func (m customApolloLogger) Warnf(fmt string, v ...interface{}) {
	klog.Warnf(fmt, v...)
}

func (m customApolloLogger) Errorf(fmt string, v ...interface{}) {
	klog.Errorf(fmt, v...)
}

func (m customApolloLogger) Debugf(fmt string, v ...interface{}) {
	klog.Debugf(fmt, v...)
}
