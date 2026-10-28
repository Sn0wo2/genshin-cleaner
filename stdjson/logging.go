package stdjson

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"os"

	"github.com/Sn0wo2/caelum"
	"github.com/Sn0wo2/genshin-cleaner/genshin"
)

type Status int

const (
	StatusLogDebug Status = 100
	StatusLogInfo  Status = 101
	StatusLogWarn  Status = 102
	StatusLogError Status = 103

	StatusResultOK      Status = 200
	StatusResultFailed  Status = 201
	StatusResultPartial Status = 202
)

const Version = "0.0.0"

const Protocol = 1

const (
	StageScan     = "scan"
	StageList     = "list"
	StageDryRun   = "dry-run"
	StageDelete   = "delete"
	StageStub     = "stub"
	StageUnstub   = "unstub"
	StageStubScan = "stub-scan"
)

type Data struct {
	V       int    `json:"v"`
	Version string `json:"version,omitempty"`
	Stage   string `json:"stage,omitempty"`
}

type Failure struct {
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason"`
}

type PathError struct {
	Path   string
	Reason string
}

func (e *PathError) Error() string { return e.Path + ": " + e.Reason }

type ErrorList struct {
	Err error
}

func (e ErrorList) MarshalJSON() ([]byte, error) {
	if e.Err == nil {
		return []byte("null"), nil
	}
	errs := []error{e.Err}
	if joined, ok := e.Err.(interface{ Unwrap() []error }); ok {
		errs = joined.Unwrap()
	}
	out := make([]Failure, 0, len(errs))
	for _, err := range errs {
		f := Failure{Reason: err.Error()}
		switch pe := err.(type) {
		case *PathError:
			f.Path, f.Reason = pe.Path, pe.Reason
		case *fs.PathError:
			f.Path, f.Reason = pe.Path, pe.Err.Error()
		}
		out = append(out, f)
	}
	return json.Marshal(out)
}

type ListData struct {
	Data
	Genshins []genshin.Genshin `json:"genshins,omitempty"`
	Failed   *ErrorList        `json:"failed,omitempty"`
}

type DryRunData struct {
	Data
	Genshin    genshin.Genshin `json:"genshin,omitempty"`
	TotalItems int             `json:"totalItems,omitempty"`
	TotalBytes int64           `json:"totalBytes,omitempty"`
	Rules      []Rule          `json:"rules,omitempty"`
	Failed     *ErrorList      `json:"failed,omitempty"`
}

type DeleteData struct {
	Data
	Genshin    genshin.Genshin `json:"genshin,omitempty"`
	TotalItems int             `json:"totalItems,omitempty"`
	TotalBytes int64           `json:"totalBytes,omitempty"`
	DoneItems  int             `json:"doneItems,omitempty"`
	FreedBytes int64           `json:"freedBytes,omitempty"`
	Failed     *ErrorList      `json:"failed,omitempty"`
}

type Rule struct {
	Id      string   `json:"id,omitempty"`
	Params  []string `json:"params,omitempty"`
	Targets []Target `json:"targets,omitempty"`
}

type Target struct {
	Path      string `json:"path,omitempty"`
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
}

func NewListData(genshins []genshin.Genshin, failed error) ListData {
	return ListData{Data{V: Protocol, Version: Version, Stage: StageList}, genshins, newErrorList(failed)}
}

func NewDryRunData(g genshin.Genshin, stage string, totalItems int, totalBytes int64, rules []Rule, failed error) DryRunData {
	return DryRunData{Data{V: Protocol, Version: Version, Stage: stage}, g, totalItems, totalBytes, rules, newErrorList(failed)}
}

func NewDeleteData(g genshin.Genshin, stage string, totalItems int, totalBytes int64, doneItems int, freedBytes int64, failed error) DeleteData {
	return DeleteData{Data{V: Protocol, Version: Version, Stage: stage}, g, totalItems, totalBytes, doneItems, freedBytes, newErrorList(failed)}
}

func newErrorList(err error) *ErrorList {
	if err == nil {
		return nil
	}
	return &ErrorList{Err: err}
}

var (
	caelumOut *caelum.Logger
	caelumErr *caelum.Logger
	debug     bool
)

func Init(debugging bool) {
	debug = debugging
	opts := slog.HandlerOptions{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case slog.TimeKey:
			return slog.Attr{}
		case slog.LevelKey:
			if lv, ok := a.Value.Any().(slog.Level); ok {
				return slog.Int("status", int(lv))
			}
		}
		return a
	}}
	caelumOut = caelum.New(caelum.Config{
		Level: slog.LevelInfo,
		Targets: []caelum.Target{{
			Writer:         &Writer{dst: os.Stdout, pretty: debug},
			Format:         caelum.JSON,
			HandlerOptions: opts,
		}},
	})

	caelumErr = caelum.New(caelum.Config{
		Level: slog.LevelInfo,
		Targets: []caelum.Target{{
			Writer:         &Writer{dst: os.Stderr, pretty: debug},
			Format:         caelum.JSON,
			HandlerOptions: opts,
		}},
	})
}

func NoData() Data { return Data{V: Protocol, Version: Version} }

func StageData(stage string) Data { return Data{V: Protocol, Version: Version, Stage: stage} }

func Log[T any](s Status, msg string, data T) {
	if s == StatusLogDebug && !debug {
		return
	}
	caelumErr.Log(context.Background(), slog.Level(s), msg, slog.Any("data", data))
}

func Result[T any](s Status, msg string, data T) {
	caelumOut.Log(context.Background(), slog.Level(s), msg, slog.Any("data", data))
}
