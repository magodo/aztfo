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
	Id     ResourceId    `json:"id"`
	Create APIOperations `json:"create,omitempty"`
	Read   APIOperations `json:"read,omitempty"`
	Update APIOperations `json:"update,omitempty"`
	Delete APIOperations `json:"delete,omitempty"`
}
