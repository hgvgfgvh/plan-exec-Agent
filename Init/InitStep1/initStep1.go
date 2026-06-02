package InitStep1

//TODO 统一触发包加载
import (
	_ "AgentTest/behavior/skill/active" // TODO 确保注册了 ChromeSkill ==【go中包不会自动加载 需要触发到它后 才会加载】
	_ "AgentTest/behavior/skill/communicationDomain"
	_ "AgentTest/behavior/skill/officeDomain"
	_ "AgentTest/behavior/skill/see" // TODO 确保注册了 ChromeSkill

	_ "AgentTest/util/tts"
)
