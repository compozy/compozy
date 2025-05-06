package main

import compozy "github.com/compozy/compozy/cmd"

func main() {
	cmd := compozy.RootCmd()
	cmd.Execute()
}
