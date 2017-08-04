package models



func StartCredits() (uint64){
	return 100
}

func BandwidthToCredits(bandwidthInMegabits uint64) (uint64){
	return bandwidthInMegabits / 10
}
func CreditsToBandwidthInMegabits(credits uint64)(uint64){
	return credits * 10
}