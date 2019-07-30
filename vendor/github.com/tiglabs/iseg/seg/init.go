package seg

import (
	"github.com/tiglabs/iseg/library"
	"github.com/tiglabs/iseg/seg/dat"
	"github.com/tiglabs/iseg/seg/recognition"
)

func init() {
	path := library.Init()
	if path == "" {
		return
	}

	dat.Init()
	recognition.InitForeign()
	recognition.InitNgram()
	recognition.InitPerson()

}
