package portal

import "AgentTest/interaction"

func init() {
	interaction.SetProcessTurn(ProcessTurn)
}
