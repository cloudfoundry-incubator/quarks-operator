Packages in pkg/bosh don't use a kubernetes client, no context.Context is passed in.
They also don't log directly, passing in a trace-level logger is possible.
