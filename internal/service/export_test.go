package service

func PlaceFlowFile(rename, link func(oldname, newname string) error, tmp, path string) (bool, error) {
	return placeFlowFile(rename, link, tmp, path)
}
