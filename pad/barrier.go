package pad

type Barrier interface {
	Load() uint64
}

type MinBarrier []Barrier

func (m MinBarrier) Load() uint64 {
	minimum := m[0].Load()
	for i := 1; i < len(m); i++ {
		if seq := m[i].Load(); seq < minimum {
			minimum = seq
		}
	}
	return minimum
}
