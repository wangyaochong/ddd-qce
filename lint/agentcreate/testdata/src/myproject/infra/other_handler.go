package infra

import agentcmd "github.com/xxx/code_evolve_agent/ddd/agent/command"

func BadHandler() {
	_ = &agentcmd.CreatePendingAgentCommand{} // want "dddagentcreate"
}
