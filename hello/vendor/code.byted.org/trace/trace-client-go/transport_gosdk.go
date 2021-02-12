package trace

import (
	"sync"
	"sync/atomic"

	"code.byted.org/gopkg/logs"
	"code.byted.org/gopkg/thrift"
	"code.byted.org/log_market/gosdk"
	"code.byted.org/trace/trace-client-go/jaeger-client"
	j "code.byted.org/trace/trace-client-go/jaeger-client/thrift-gen/jaeger"
	"github.com/pkg/errors"
)

const (
	defaultTaskName = "opentracing"
)

var defaultTags = map[string]string{
	"_data_fmt": "tracing",
	"_protocol": "thrift",
	"_psm":      "toutiao.unknown.unknown",
}

type gosdkSender struct {
	// These fields must be first in the struct because `sync/atomic` expects 64-bit alignment.
	// Cf. https://github.com/golang/go/issues/13868
	closed int64 // 0 - not closed, 1 - closed

	sync.Mutex
	taskName       string     // task name for gosdk send api
	process        *j.Process // process info
	serializerPool sync.Pool
}

// NewGosdkTransport creates a reporter that submits spans to jaeger-agent
func NewGosdkTransport(taskName string) (jaeger.Transport, error) {
	if taskName == "" {
		taskName = defaultTaskName
	}

	sender := &gosdkSender{
		taskName: taskName,
		serializerPool: sync.Pool{New: func() interface{} {
			return thrift.NewTSerializer()
		}},
	}
	return sender, nil
}

func (s *gosdkSender) Append(span *jaeger.Span) (int, error) {
	if s.process == nil {
		s.Lock()
		if s.process == nil {
			s.process = jaeger.BuildJaegerProcessThrift(span)
			defaultTags["_psm"] = s.process.ServiceName
		}
		s.Unlock()
	}

	jSpan := jaeger.BuildJaegerThrift(span)
	serializer := s.serializerPool.Get().(*thrift.TSerializer)
	defer s.serializerPool.Put(serializer)

	data, err := serializer.Write(jSpan)
	if err != nil {
		logs.Warn("serialize for span generated by %s::%s failed.",
			s.process.ServiceName, jSpan.OperationName)
		return 1, err
	}

	// TODO(zhanggongyuan): add extra tags for dispatch
	msg := &gosdk.Msg{
		Msg:  data,
		Tags: defaultTags,
	}
	if err = gosdk.Send(s.taskName, msg); err != nil {
		logs.Warn("gosdk send msg for %s::%s failed",
			s.process.ServiceName, jSpan.OperationName)
		return 1, err
	}

	return 1, nil
}

func (s *gosdkSender) Flush() (int, error) {
	return 0, nil
}

func (s *gosdkSender) Close() error {
	if swapped := atomic.CompareAndSwapInt64(&s.closed, 0, 1); !swapped {
		return errors.Errorf("repeated attempt to close the sender is ignored")
	}
	_, err := s.Flush()

	return err
}
