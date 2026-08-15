// internal/doctor/doctor.go
package doctor

import (
	"os"
	"runtime"
	"time"

	"github.com/Bissiking/argos-prob/internal/capabilities"
	"github.com/Bissiking/argos-prob/internal/config"
)

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type Report struct {
	CheckedAt time.Time                 `json:"checked_at"`
	OS        string                    `json:"os"`
	Arch      string                    `json:"arch"`
	Checks    []Check                   `json:"checks"`
	Features  capabilities.Capabilities `json:"capabilities"`
}

func Run() Report {
	report := Report{CheckedAt: time.Now().UTC(), OS: runtime.GOOS, Arch: runtime.GOARCH, Features: capabilities.Detect()}
	if _, err := os.Hostname(); err != nil {
		report.Checks = append(report.Checks, Check{Name: "hostname", OK: false, Message: err.Error()})
	} else {
		report.Checks = append(report.Checks, Check{Name: "hostname", OK: true, Message: "hostname available"})
	}
	if path, err := config.Path(); err != nil {
		report.Checks = append(report.Checks, Check{Name: "config", OK: false, Message: err.Error()})
	} else {
		report.Checks = append(report.Checks, Check{Name: "config", OK: true, Message: path})
	}
	return report
}
