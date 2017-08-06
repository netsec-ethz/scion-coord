package models



func StartCredits() (int64){
	return 100
}

func BandwidthToCredits(bandwidthInKilobits uint64) (int64){
	return int64(bandwidthInKilobits) / 1000 / 10
}
func CreditsToBandwidthInBandwidthInKilobits(credits uint64)(int64){
	return int64(credits) * 1000 * 10
}