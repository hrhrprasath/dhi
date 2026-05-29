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
	CPU       float64
	RAMUsedMB uint64
	RAMTotalMB uint64
	UptimeSec  uint64
	Hostname   string
	Kernel     string
	GPU        string
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

	m.GPU = getGPUInfo()
	return m
}

func getGPUInfo() string {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Chipset Model:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Chipset Model:"))
		}
	}
	return ""
}
