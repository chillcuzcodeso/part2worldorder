package heartbeat

import (
	"bytes"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	"part2worldorder/internal/config"
	"part2worldorder/internal/sysinfo"
)

const (
	baseInterval    = 60 * time.Second
	jitterSeconds   = 10
	unavailableWait = 2 * time.Minute
)

type payload struct {
	ClientID  string  `json:"client_id"`
	Hostname  string  `json:"hostname"`
	OSVersion string  `json:"os_version"`
	CPULoad   float64 `json:"cpu_load"`
}

// Run sends periodic heartbeat payloads until the process exits.
func Run(client *http.Client, clientID string) {
	for {
		if send(client, clientID) == http.StatusServiceUnavailable {
			time.Sleep(unavailableWait)
			continue
		}
		time.Sleep(nextInterval())
	}
}

func nextInterval() time.Duration {
	offset := rand.IntN(jitterSeconds*2+1) - jitterSeconds
	return baseInterval + time.Duration(offset)*time.Second
}

func send(client *http.Client, clientID string) int {
	body, err := json.Marshal(collect(clientID))
	if err != nil {
		return 0
	}

	req, err := http.NewRequest(http.MethodPost, config.ServerBase()+"/v1/heartbeat", bytes.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", clientID)

	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode
}

func collect(clientID string) payload {
	host, _ := os.Hostname()
	return payload{
		ClientID:  clientID,
		Hostname:  host,
		OSVersion: sysinfo.OSVersion(),
		CPULoad:   sysinfo.CPULoad(),
	}
}
