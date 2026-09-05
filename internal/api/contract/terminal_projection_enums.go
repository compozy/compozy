package contract

import terminalpkg "github.com/compozy/compozy/internal/terminal"

func TerminalModeValues() []string {
	return []string{string(terminalpkg.ModePTY), string(terminalpkg.ModePipe)}
}

func TerminalActorKindValues() []string {
	return []string{
		string(terminalpkg.ActorKindHuman),
		string(terminalpkg.ActorKindAgent),
		string(terminalpkg.ActorKindSystem),
	}
}

func TerminalSignalValues() []string {
	return []string{
		string(terminalpkg.SignalINT),
		string(terminalpkg.SignalTERM),
		string(terminalpkg.SignalKILL),
		string(terminalpkg.SignalHUP),
	}
}
