package server

import agentcmd "github.com/xxx/code_evolve_agent/ddd/agent/command"

func AgentExecute() {
	_ = &agentcmd.CreatePendingAgentCommand{}
}
