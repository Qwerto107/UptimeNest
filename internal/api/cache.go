package api

func publicHeartbeatBucket(count int) int {
	for _, bucket := range []int{30, 60, 90, 180, 360, 720, 1000} {
		if count <= bucket {
			return bucket
		}
	}
	return 1000
}
