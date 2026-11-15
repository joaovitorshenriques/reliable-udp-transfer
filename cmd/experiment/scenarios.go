package main

import (
	"time"
	"trabalho_redes/internal/netem"
)

func scenarios(window int, seed int64) []group {
	rtts := []time.Duration{25 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond, 400 * time.Millisecond}
	losses := []float64{0, 0, 0.05, 0.05}
	result := make([]group, 4)
	for i := range result {
		result[i] = group{Name: []string{"S1", "S2", "S3", "S4"}[i], Adaptive: true, Buffer: 4 * 1024 * 1024, RTT: rtts[i], Loss: losses[i], Window: window, Seed: seed + int64(i)*1000000}
	}
	return result
}

// Counters cover the full upload/download pair, including requests and linger.
type emulationRecord struct {
	Group          string      `json:"group"`
	Repetition     int         `json:"repetition"`
	RTTMillis      float64     `json:"added_rtt_ms"`
	Loss           float64     `json:"loss_probability_per_direction"`
	Seed           int64       `json:"client_seed"`
	Window         int         `json:"window_packets"`
	ClientToServer netem.Stats `json:"client_to_server"`
	ServerToClient netem.Stats `json:"server_to_client"`
}
