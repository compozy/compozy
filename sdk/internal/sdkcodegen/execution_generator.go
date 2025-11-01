package sdkcodegen

import (
	"fmt"
	"strings"

	"github.com/dave/jennifer/jen"
)

func buildExecutionFile() *jen.File {
	f := jen.NewFile(packageName)
	addExecutionImports(f)
	emitExecutionFunctions(f)
	addExecutionHelpers(f)
	return f
}

func addExecutionImports(f *jen.File) {
	f.ImportName("context", "context")
	f.ImportName("fmt", "fmt")
	f.ImportName("strconv", "strconv")
	f.ImportName("strings", "strings")
	f.ImportName("time", "time")
	f.ImportAlias("github.com/compozy/compozy/engine/core", "core")
	f.ImportAlias("github.com/compozy/compozy/sdk/v2/client", "client")
}

type execSpec struct {
	Resource         string
	IDParam          string
	BuildRequestFunc string
	BuildSyncFunc    string
	ClientExecute    string
	ClientSync       string
	ClientStream     string
}

func emitExecutionFunctions(f *jen.File) {
	specs := []execSpec{
		{
			Resource:         "Workflow",
			IDParam:          "workflowID",
			BuildRequestFunc: "buildWorkflowExecuteRequest",
			BuildSyncFunc:    "buildWorkflowSyncRequest",
			ClientExecute:    "ExecuteWorkflow",
			ClientSync:       "ExecuteWorkflowSync",
			ClientStream:     "ExecuteWorkflowStream",
		},
		{
			Resource:         "Task",
			IDParam:          "taskID",
			BuildRequestFunc: "buildTaskExecuteRequest",
			BuildSyncFunc:    "buildTaskSyncRequest",
			ClientExecute:    "ExecuteTask",
			ClientSync:       "ExecuteTaskSync",
			ClientStream:     "ExecuteTaskStream",
		},
		{
			Resource:         "Agent",
			IDParam:          "agentID",
			BuildRequestFunc: "buildAgentExecuteRequest",
			BuildSyncFunc:    "buildAgentSyncRequest",
			ClientExecute:    "ExecuteAgent",
			ClientSync:       "ExecuteAgentSync",
			ClientStream:     "ExecuteAgentStream",
		},
	}
	for i := range specs {
		spec := &specs[i]
		addExecuteFunction(f, spec)
		addExecuteSyncFunction(f, spec)
		addExecuteStreamFunction(f, spec)
	}
}

func addExecuteFunction(f *jen.File, spec *execSpec) {
	f.Comment(
		fmt.Sprintf(
			"Execute%s triggers asynchronous %s execution via the client.",
			spec.Resource,
			strings.ToLower(spec.Resource),
		),
	)
	f.Func().
		Params(jen.Id("e").Op("*").Id("Engine")).
		Id(fmt.Sprintf("Execute%s", spec.Resource)).
		Params(
			jen.Id("ctx").Qual("context", "Context"),
			jen.Id(spec.IDParam).String(),
			jen.Id("req").Op("*").Id("ExecuteRequest"),
		).
		Params(
			jen.Op("*").Id("ExecuteResponse"),
			jen.Error(),
		).
		BlockFunc(func(g *jen.Group) {
			g.List(jen.Id("cli"), jen.Err()).Op(":=").Id("ensureClient").Call(jen.Id("e"))
			g.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Err()),
			)
			g.Id("payload").Op(":=").Id(spec.BuildRequestFunc).Call(jen.Id("req"))
			g.List(jen.Id("resp"), jen.Err()).Op(":=").Id("cli").Dot(spec.ClientExecute).Call(
				jen.Id("ctx"),
				jen.Id(spec.IDParam),
				jen.Id("payload"),
			)
			g.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Err()),
			)
			g.Return(
				jen.Id("newExecuteResponse").Call(
					jen.Id("resp").Dot("ExecID"),
					jen.Id("resp").Dot("ExecURL"),
				),
				jen.Nil(),
			)
		})
}

func addExecuteSyncFunction(f *jen.File, spec *execSpec) {
	f.Comment(
		fmt.Sprintf(
			"Execute%sSync performs synchronous %s execution and returns the result.",
			spec.Resource,
			strings.ToLower(spec.Resource),
		),
	)
	f.Func().
		Params(jen.Id("e").Op("*").Id("Engine")).
		Id(fmt.Sprintf("Execute%sSync", spec.Resource)).
		Params(
			jen.Id("ctx").Qual("context", "Context"),
			jen.Id(spec.IDParam).String(),
			jen.Id("req").Op("*").Id("ExecuteSyncRequest"),
		).
		Params(
			jen.Op("*").Id("ExecuteSyncResponse"),
			jen.Error(),
		).
		BlockFunc(func(g *jen.Group) {
			g.List(jen.Id("cli"), jen.Err()).Op(":=").Id("ensureClient").Call(jen.Id("e"))
			g.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Err()),
			)
			g.Id("payload").Op(":=").Id(spec.BuildSyncFunc).Call(jen.Id("req"))
			g.List(jen.Id("resp"), jen.Err()).Op(":=").Id("cli").Dot(spec.ClientSync).Call(
				jen.Id("ctx"),
				jen.Id(spec.IDParam),
				jen.Id("payload"),
			)
			g.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Err()),
			)
			g.Return(
				jen.Id("buildSyncResponse").Call(
					jen.Id("resp").Dot("ExecID"),
					jen.Id("resp").Dot("Output"),
				),
				jen.Nil(),
			)
		})
}

func addExecuteStreamFunction(f *jen.File, spec *execSpec) {
	f.Comment(
		fmt.Sprintf(
			"Execute%sStream starts %s execution and returns a stream session.",
			spec.Resource,
			strings.ToLower(spec.Resource),
		),
	)
	f.Func().
		Params(jen.Id("e").Op("*").Id("Engine")).
		Id(fmt.Sprintf("Execute%sStream", spec.Resource)).
		Params(
			jen.Id("ctx").Qual("context", "Context"),
			jen.Id(spec.IDParam).String(),
			jen.Id("req").Op("*").Id("ExecuteRequest"),
			jen.Id("opts").Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "StreamOptions"),
		).
		Params(
			jen.Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "StreamSession"),
			jen.Error(),
		).
		BlockFunc(func(g *jen.Group) {
			g.List(jen.Id("cli"), jen.Err()).Op(":=").Id("ensureClient").Call(jen.Id("e"))
			g.If(jen.Err().Op("!=").Nil()).Block(
				jen.Return(jen.Nil(), jen.Err()),
			)
			g.Id("payload").Op(":=").Id(spec.BuildRequestFunc).Call(jen.Id("req"))
			g.Return(
				jen.Id("cli").Dot(spec.ClientStream).Call(
					jen.Id("ctx"),
					jen.Id(spec.IDParam),
					jen.Id("payload"),
					jen.Id("opts"),
				),
			)
		})
}

func addExecutionHelpers(f *jen.File) {
	addEnsureClientHelper(f)
	addNewExecuteResponseHelper(f)
	addBuildSyncResponseHelper(f)
	addCopyInputHelper(f)
	addCopyOutputHelper(f)
	addStringFromOptionsHelper(f)
	addIntFromOptionsHelper(f)
	addDurationSecondsHelper(f)
	addWorkflowRequestHelpers(f)
	addTaskRequestHelpers(f)
	addAgentRequestHelpers(f)
}

func addEnsureClientHelper(f *jen.File) {
	f.Func().
		Id("ensureClient").
		Params(jen.Id("e").Op("*").Id("Engine")).
		Params(
			jen.Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "Client"),
			jen.Error(),
		).
		Block(
			jen.If(jen.Id("e").Op("==").Nil()).Block(
				jen.Return(jen.Nil(), jen.Qual("fmt", "Errorf").Call(jen.Lit("engine is nil"))),
			),
			jen.If(jen.Id("e").Dot("client").Op("==").Nil()).Block(
				jen.Return(jen.Nil(), jen.Qual("fmt", "Errorf").Call(jen.Lit("engine client is not initialized"))),
			),
			jen.Return(jen.Id("e").Dot("client"), jen.Nil()),
		)
}

func addNewExecuteResponseHelper(f *jen.File) {
	f.Func().
		Id("newExecuteResponse").
		Params(
			jen.Id("execID").String(),
			jen.Id("execURL").String(),
		).
		Op("*").Id("ExecuteResponse").
		Block(
			jen.Return(jen.Op("&").Id("ExecuteResponse").Values(jen.Dict{
				jen.Id("ExecID"):  jen.Id("execID"),
				jen.Id("ExecURL"): jen.Id("execURL"),
			})),
		)
}

func addBuildSyncResponseHelper(f *jen.File) {
	f.Func().
		Id("buildSyncResponse").
		Params(
			jen.Id("execID").String(),
			jen.Id("output").Op("*").Qual("github.com/compozy/compozy/engine/core", "Output"),
		).
		Op("*").Id("ExecuteSyncResponse").
		Block(
			jen.Return(jen.Op("&").Id("ExecuteSyncResponse").Values(jen.Dict{
				jen.Id("ExecID"): jen.Id("execID"),
				jen.Id("Output"): jen.Id("copyOutput").Call(jen.Id("output")),
			})),
		)
}

func addCopyInputHelper(f *jen.File) {
	f.Func().
		Id("copyInput").
		Params(jen.Id("values").Map(jen.String()).Any()).
		Qual("github.com/compozy/compozy/engine/core", "Input").
		Block(
			jen.If(jen.Len(jen.Id("values")).Op("==").Lit(0)).Block(
				jen.Return(jen.Nil()),
			),
			jen.Id("cloned").Op(":=").Qual("github.com/compozy/compozy/engine/core", "CopyMaps").Call(jen.Id("values")),
			jen.If(jen.Len(jen.Id("cloned")).Op("==").Lit(0)).Block(
				jen.Return(jen.Nil()),
			),
			jen.Return(jen.Qual("github.com/compozy/compozy/engine/core", "Input").Call(jen.Id("cloned"))),
		)
}

func addCopyOutputHelper(f *jen.File) {
	f.Func().
		Id("copyOutput").
		Params(jen.Id("output").Op("*").Qual("github.com/compozy/compozy/engine/core", "Output")).
		Map(jen.String()).Any().
		Block(
			jen.If(jen.Id("output").Op("==").Nil()).Block(
				jen.Return(jen.Nil()),
			),
			jen.Return(jen.Id("output").Dot("AsMap").Call()),
		)
}

func addStringFromOptionsHelper(f *jen.File) {
	f.Func().
		Id("stringFromOptions").
		Params(
			jen.Id("options").Map(jen.String()).Any(),
			jen.Id("key").String(),
		).
		String().
		BlockFunc(func(g *jen.Group) {
			g.If(jen.Id("options").Op("==").Nil()).Block(jen.Return(jen.Lit("")))
			g.Id("raw").Op(",").Id("ok").Op(":=").Id("options").Index(jen.Id("key"))
			g.If(jen.Op("!").Id("ok")).Block(jen.Return(jen.Lit("")))
			g.Id("str").Op(",").Id("isString").Op(":=").Id("raw").Assert(jen.String())
			g.If(jen.Id("isString")).Block(
				jen.Return(jen.Qual("strings", "TrimSpace").Call(jen.Id("str"))),
			)
			g.Id("stringer").Op(",").Id("isStringer").Op(":=").Id("raw").Assert(jen.Qual("fmt", "Stringer"))
			g.If(jen.Id("isStringer")).Block(
				jen.Return(jen.Qual("strings", "TrimSpace").Call(jen.Id("stringer").Dot("String").Call())),
			)
			g.Return(jen.Lit(""))
		})
}

func addIntFromOptionsHelper(f *jen.File) {
	f.Func().
		Id("intFromOptions").
		Params(
			jen.Id("options").Map(jen.String()).Any(),
			jen.Id("key").String(),
		).
		Op("*").Int().
		BlockFunc(func(g *jen.Group) {
			g.If(jen.Id("options").Op("==").Nil()).Block(jen.Return(jen.Nil()))
			g.Id("raw").Op(",").Id("ok").Op(":=").Id("options").Index(jen.Id("key"))
			g.If(jen.Op("!").Id("ok")).Block(jen.Return(jen.Nil()))
			g.Var().Id("value").Int()
			g.Id("intValue").Op(",").Id("isInt").Op(":=").Id("raw").Assert(jen.Int())
			g.If(jen.Id("isInt")).Block(
				jen.Id("value").Op("=").Id("intValue"),
			).Else().BlockFunc(func(b *jen.Group) {
				b.Id("strValue").Op(",").Id("isString").Op(":=").Id("raw").Assert(jen.String())
				b.If(jen.Op("!").Id("isString")).Block(jen.Return(jen.Nil()))
				b.Id("parsed").
					Op(",").
					Id("err").
					Op(":=").
					Qual("strconv", "Atoi").
					Call(jen.Qual("strings", "TrimSpace").Call(jen.Id("strValue")))
				b.If(jen.Err().Op("!=").Nil()).Block(jen.Return(jen.Nil()))
				b.Id("value").Op("=").Id("parsed")
			})
			g.If(jen.Id("value").Op("<=").Lit(0)).Block(jen.Return(jen.Nil()))
			g.Return(jen.Op("&").Id("value"))
		})
}

func addDurationSecondsHelper(f *jen.File) {
	f.Func().
		Id("durationSeconds").
		Params(jen.Id("value").Op("*").Qual("time", "Duration")).
		Op("*").Int().
		Block(
			jen.If(jen.Id("value").Op("==").Nil()).Block(jen.Return(jen.Nil())),
			jen.Id("secs").Op(":=").Id("int").Call(jen.Id("value").Dot("Seconds").Call()),
			jen.If(jen.Id("secs").Op("<=").Lit(0)).Block(jen.Return(jen.Nil())),
			jen.Return(jen.Op("&").Id("secs")),
		)
}

func addWorkflowRequestHelpers(f *jen.File) {
	f.Func().
		Id("buildWorkflowExecuteRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "WorkflowExecuteRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "WorkflowExecuteRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("inputCopy").Op(":=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.If(jen.Id("inputCopy").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Input").Op("=").Id("inputCopy"),
			)
			g.Id("task").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("task_id"))
			g.If(jen.Id("task").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("TaskID").Op("=").Id("task"),
			)
			g.Return(jen.Id("payload"))
		})

	f.Func().
		Id("buildWorkflowSyncRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteSyncRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "WorkflowSyncRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "WorkflowSyncRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("inputCopy").Op(":=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.If(jen.Id("inputCopy").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Input").Op("=").Id("inputCopy"),
			)
			g.Id("secs").Op(":=").Id("durationSeconds").Call(jen.Id("req").Dot("Timeout"))
			g.If(jen.Id("secs").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Op("*").Id("secs"),
			)
			g.Id("task").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("task_id"))
			g.If(jen.Id("task").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("TaskID").Op("=").Id("task"),
			)
			g.Return(jen.Id("payload"))
		})
}

func addTaskRequestHelpers(f *jen.File) {
	f.Func().
		Id("buildTaskExecuteRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "TaskExecuteRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "TaskExecuteRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("payload").Dot("With").Op("=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.Id("timeoutOpt").Op(":=").Id("intFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("timeout"))
			g.If(jen.Id("timeoutOpt").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Id("timeoutOpt"),
			)
			g.Return(jen.Id("payload"))
		})

	f.Func().
		Id("buildTaskSyncRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteSyncRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "TaskExecuteRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "TaskExecuteRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("payload").Dot("With").Op("=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.Id("secs").Op(":=").Id("durationSeconds").Call(jen.Id("req").Dot("Timeout"))
			g.If(jen.Id("secs").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Id("secs"),
				jen.Return(jen.Id("payload")),
			)
			g.Id("timeoutOpt").Op(":=").Id("intFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("timeout"))
			g.If(jen.Id("timeoutOpt").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Id("timeoutOpt"),
			)
			g.Return(jen.Id("payload"))
		})
}

func addAgentRequestHelpers(f *jen.File) {
	addAgentExecuteRequestBuilder(f)
	addAgentSyncRequestBuilder(f)
}

func addAgentExecuteRequestBuilder(f *jen.File) {
	f.Func().
		Id("buildAgentExecuteRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "AgentExecuteRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "AgentExecuteRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("payload").Dot("With").Op("=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.Id("action").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("action"))
			g.If(jen.Id("action").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("Action").Op("=").Id("action"),
			)
			g.Id("prompt").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("prompt"))
			g.If(jen.Id("prompt").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("Prompt").Op("=").Id("prompt"),
			)
			g.Id("timeoutOpt").Op(":=").Id("intFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("timeout"))
			g.If(jen.Id("timeoutOpt").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Op("*").Id("timeoutOpt"),
			)
			g.Return(jen.Id("payload"))
		})
}

func addAgentSyncRequestBuilder(f *jen.File) {
	f.Func().
		Id("buildAgentSyncRequest").
		Params(jen.Id("req").Op("*").Id("ExecuteSyncRequest")).
		Op("*").Qual("github.com/compozy/compozy/sdk/v2/client", "AgentExecuteRequest").
		BlockFunc(func(g *jen.Group) {
			g.Id("payload").
				Op(":=").
				Op("&").
				Qual("github.com/compozy/compozy/sdk/v2/client", "AgentExecuteRequest").
				Values()
			g.If(jen.Id("req").Op("==").Nil()).Block(jen.Return(jen.Id("payload")))
			g.Id("payload").Dot("With").Op("=").Id("copyInput").Call(jen.Id("req").Dot("Input"))
			g.Id("action").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("action"))
			g.If(jen.Id("action").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("Action").Op("=").Id("action"),
			)
			g.Id("prompt").Op(":=").Id("stringFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("prompt"))
			g.If(jen.Id("prompt").Op("!=").Lit("")).Block(
				jen.Id("payload").Dot("Prompt").Op("=").Id("prompt"),
			)
			g.Id("secs").Op(":=").Id("durationSeconds").Call(jen.Id("req").Dot("Timeout"))
			g.If(jen.Id("secs").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Op("*").Id("secs"),
				jen.Return(jen.Id("payload")),
			)
			g.Id("timeoutOpt").Op(":=").Id("intFromOptions").Call(jen.Id("req").Dot("Options"), jen.Lit("timeout"))
			g.If(jen.Id("timeoutOpt").Op("!=").Nil()).Block(
				jen.Id("payload").Dot("Timeout").Op("=").Op("*").Id("timeoutOpt"),
			)
			g.Return(jen.Id("payload"))
		})
}
