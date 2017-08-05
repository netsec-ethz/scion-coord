package models



func StartCredits() (int64){
	return 100
}

func BandwidthToCredits(bandwidthInMegabits uint64) (int64){
	return int64(bandwidthInMegabits) / 10
}
func CreditsToBandwidthInMegabits(credits uint64)(int64){
	return int64(credits) * 10
}