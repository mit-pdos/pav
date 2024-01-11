package full

import (
	"github.com/tchajed/goose/machine"
)

const aliceMsg uint64 = 10
const bobMsg uint64 = 11

func alice(c *ChatCli) {
	a_msg := &msgT{body: aliceMsg}
	b_msg := &msgT{body: bobMsg}
	c.Put(a_msg)

	g := c.Get()
	if 2 <= len(g) {
		machine.Assert(g[0].body == a_msg.body)
		machine.Assert(g[1].body == b_msg.body)
		machine.Assert(len(g) == 2)

		g2 := c.Get()
		machine.Assert(g2[0].body == a_msg.body)
		machine.Assert(g2[1].body == b_msg.body)
		machine.Assert(len(g2) == 2)
	}
}

func bob(c *ChatCli) {
	a_msg := &msgT{body: aliceMsg}
	b_msg := &msgT{body: bobMsg}
	g := c.Get()
	if 1 <= len(g) {
		machine.Assert(g[0].body == a_msg.body)
		machine.Assert(len(g) == 1)
		c.Put(b_msg)
	}
}

func main() {
	c := Init()
	go func() { alice(c) }()
	bob(c)
}