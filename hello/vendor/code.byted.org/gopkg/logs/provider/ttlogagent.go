package provider

import (
	"fmt"
	"time"
	"code.byted.org/gopkg/logs"
	"code.byted.org/log_market/ttlogagent_gosdk"
	"code.byted.org/gopkg/logs/utils"
)

var (
	//copy from code.byted.org/gopkg/logs
	levelStrings = []string{
		"Trace",
		"Debug",
		"Info",
		"Notice",
		"Warn",
		"Error",
		"Fatal",
	}
)

type LogAgentProvider struct {
	level  int
	psm    string
	sender *ttlogagent_gosdk.LogSender
}

func NewLogAgentProvider(psm string) *LogAgentProvider {
	return &LogAgentProvider{
		psm: psm,
	}
}

func (p *LogAgentProvider) Init() error {
	s := ttlogagent_gosdk.NewLogSenderByName(p.psm)
	s.Start()
	p.sender = s
	return nil
}

func (p *LogAgentProvider) SetLevel(level int) {
	p.level = level
}

//not implements
func (p *LogAgentProvider) WriteMsg(msg string, level int) error {
	return nil
}

func (p *LogAgentProvider) Destroy() error {
	p.sender.Exit()
	return nil
}

func (p *LogAgentProvider) Flush() error {
	return nil
}

func (p *LogAgentProvider) Level() int {
	return p.level
}

//this function will block, if sender's channel full.
//because gopkg/logs.Logger is asynchronous, so it isn't  block user code.
func (p *LogAgentProvider) Write(log *logs.LogMsg) error {
	if log.Level < p.level {
		return nil
	}

	levelStr := ""
	if log.Level >= len(levelStrings) || log.Level < 0 {
		levelStr = fmt.Sprintf("level(%d)", log.Level)
	}
	levelStr = levelStrings[log.Level]
	header := &ttlogagent_gosdk.MsgV3Header{
		Level:    levelStr,
		Location: log.Location,
		LogID:    utils.LogIDFromContext(log.Ctx),
		Ts:       log.Time.UnixNano() / 1e6,
		SpanID:   utils.SpanIDFromContext(log.Ctx),
	}
	var kvs []string
	if len(log.Kvs) > 0 {
		kvs = make([]string, 0, len(log.Kvs))
		for i := range log.Kvs {
			kvs = append(kvs, utils.Value2Str(log.Kvs[i]))
		}
	}
	msg := ttlogagent_gosdk.NewMsgV3([]byte(log.Msg), header, kvs...)

	err := p.sender.Send(msg)

	for err == ttlogagent_gosdk.ErrChannelFull {
		time.Sleep(time.Millisecond)
		err = p.sender.Send(msg)
	}
	return err
}
