package main

type Results []Result

func (r Results) Len() int {
	return len(r)
}

func (r Results) Less(i int, j int) bool {
	return r[i].Id.String() < r[j].Id.String()
}

func (r Results) Swap(i int, j int) {
	r[i], r[j] = r[j], r[i]
}

type Result struct {
	Id     ResourceId     `json:"id"`
	Create []APIOperation `json:"create,omitempty"`
	Read   []APIOperation `json:"read,omitempty"`
	Update []APIOperation `json:"update,omitempty"`
	Delete []APIOperation `json:"delete,omitempty"`
}
