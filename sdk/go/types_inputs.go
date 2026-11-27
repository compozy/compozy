package compozysdk

import "github.com/compozy/compozy/sdk/go/contracts"

// InputsMinCompozyVersion is the first CompozyOS release that builds and delivers SDK-declared
// subprocess inputs. Describe raises the emitted floor to it when the definition declares inputs.
const InputsMinCompozyVersion = "0.7.9"

// Input declares one typed install input with the manifest [[inputs]] grammar.
type Input = contracts.DescribeInput

// InputBinding names the environment variable receiving an input value.
type InputBinding = contracts.DescribeInputBinding
