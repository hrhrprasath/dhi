package services

import (
	"os/exec"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemService struct{}

type SystemMetrics struct {
	CPU        float64
	RAMUsedMB  uint64
	RAMTotalMB uint64
	UptimeSec  uint64
	Hostname   string
	Kernel     string
	GPUPercent float64
}

func NewSystemService() *SystemService {
	return &SystemService{}
}

func (s *SystemService) GetMetrics() SystemMetrics {
	m := SystemMetrics{}

	pct, _ := cpu.Percent(0, false)
	if len(pct) > 0 {
		m.CPU = pct[0]
	}

	v, _ := mem.VirtualMemory()
	if v != nil {
		m.RAMUsedMB = v.Used / 1024 / 1024
		m.RAMTotalMB = v.Total / 1024 / 1024
	}

	hi, _ := host.Info()
	if hi != nil {
		m.Hostname = hi.Hostname
		m.Kernel = hi.KernelVersion
		m.UptimeSec = hi.Uptime
	}

	m.GPUPercent = getGPUPercent()
	return m
}

func getGPUPercent() float64 {
	out, err := exec.Command("ioreg", "-l").Output()
	if err != nil {
		return 0
	}
	raw := string(out)
	idx := strings.Index(raw, "Device Utilization %")
	if idx < 0 {
		return 0
	}
	// Find the number after '='
	rest := raw[idx:]
	eqIdx := strings.IndexByte(rest, '=')
	if eqIdx < 0 {
		return 0
	}
	var val float64
	n := 0
	for _, ch := range rest[eqIdx+1:] {
		if ch >= '0' && ch <= '9' {
			val = val*10 + float64(ch-'0')
			n++
		} else if n > 0 {
			break
		}
	}
	return val
}
