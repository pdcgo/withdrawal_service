package datasource

type OrderRefList []string

func (d *OrderRefList) Add(str string) {
	switch str {
	case "", "-":
		return
	default:
		*d = append(*d, str)
	}
}
