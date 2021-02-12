package conf

import "git.byted.org/ttgame/platform/nvwa/environ"

func RebuildPSM(psm string) string {
	// 判断当前是否为沙箱环境，返回对应的psm
	if environ.IsSandbox() {
		return psm + "_sandbox"
	}
	return psm
}

func GetDocI18nPSM() string {
	return RebuildPSM("ttgame.platform.doc_i18n")
}

func GetRoleManagerPSM() string {
	return RebuildPSM("ttgame.platform.role_manager")
}

func GetVideoRecordPSM() string {
	return RebuildPSM("ttgame.platform.video_record")
}

func GetWalletPSM() string {
	return RebuildPSM("ttgame.platform.wallet")
}

func GetSdkRpcPSM() string {
	return RebuildPSM("ttgame.platform.sdk_rpc")
}

func GetRuleCuterPSM() string {
	return RebuildPSM("ttgame.platform.rule_cuter")
}

func GetGsdkWhiteRpcPSM() string {
	return RebuildPSM("ttgame.platform.gsdk_white")
}
