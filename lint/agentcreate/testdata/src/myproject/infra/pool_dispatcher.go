package infra

import agentcmd "github.com/xxx/code_evolve_agent/ddd/agent/command"

func Dispatch() {
	_ = &agentcmd.CreatePendingAgentCommand{}
	_ = &agentcmd.CreatePendingAgentResult{}
}
