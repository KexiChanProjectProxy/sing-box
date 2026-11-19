package group

// A latency_offset is a configured number of milliseconds added to a member's
// latency when, and only when, the group ranks its members. A positive offset
// makes a member less preferred, a negative one more preferred. Health checks,
// the unhealthy timeout, stored URL test results and everything reported to
// the Clash API keep reading the measured latency.
//
// Offsets can be negative and larger than the latency they shift, so ranked
// latencies are computed as int64 rather than in the uint16 the measurements
// come in.

// offsetLatency is a measured delay shifted by a latency_offset.
func offsetLatency(delay uint16, offset int32) int64 {
	return int64(delay) + int64(offset)
}

// rankLatency is the delay the group ranks the candidate on.
func (c Candidate) rankLatency() int64 {
	return offsetLatency(c.Latency, c.Offset)
}

// applyLatencyOffsets adds each candidate's offset to the score the sorter gave
// it, and records the offset in the member's breakdown for debug logging.
func applyLatencyOffsets(candidates []Candidate, breakdowns []breakdown) {
	for i := range candidates {
		offset := candidates[i].Offset
		if offset == 0 {
			continue
		}
		candidates[i].Score += float64(offset)
		if i < len(breakdowns) {
			breakdowns[i].Score = candidates[i].Score
			breakdowns[i].Contributions = append(breakdowns[i].Contributions, contribution{
				Key:      "latency_offset",
				Value:    float64(offset),
				Measured: true,
				Weighted: float64(offset),
			})
		}
	}
}
