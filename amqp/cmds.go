package grid_amqp

import (
	//IP "3grid/ip"
	RT "3grid/route"
	//G "3grid/tools/globals"
	G "3grid/tools/globals"
	"errors"
	"fmt"
)

type Cmds struct {
}

func (c *Cmds) Ka(msg *AMQP_Message) error {
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Ka cmd.."))
	return nil
}

func (c *Cmds) Add(msg *AMQP_Message) (err error) {
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Add cmd.."))
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Adding: %+v", (*msg.Msg1)))
	return nil
}

func (c *Cmds) Ver(msg *AMQP_Message) error {
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Ver cmd.."))
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Version: %s", (*msg.Params)["ver"]))
	return nil
}

func (c *Cmds) Update(msg *AMQP_Message) (err error) {
	defer func() {
		if pan := recover(); pan != nil {
			msg := fmt.Sprintf("%s", pan)
			err = errors.New(msg)
		}
	}()
	db_type := msg.Object
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}

	return err
}
