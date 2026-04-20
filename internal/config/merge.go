package config

// MergeSSHOptions merges global and local SSHOptions. Local values override
// global values. For maps (EnvVars, ExtraOptions), entries are merged with
// local keys winning. For slices (LocalForward, RemoteForward), local
// replaces global entirely if non-nil.
func MergeSSHOptions(global, local *SSHOptions) SSHOptions {
	if global == nil && local == nil {
		return SSHOptions{}
	}
	if global == nil {
		return *local
	}
	if local == nil {
		return *global
	}

	result := *global

	if local.ForwardAgent != nil {
		result.ForwardAgent = local.ForwardAgent
	}
	if local.Compression != nil {
		result.Compression = local.Compression
	}
	if local.ServerAliveInterval != nil {
		result.ServerAliveInterval = local.ServerAliveInterval
	}
	if local.ServerAliveCountMax != nil {
		result.ServerAliveCountMax = local.ServerAliveCountMax
	}
	if local.StrictHostKeyCheck != "" {
		result.StrictHostKeyCheck = local.StrictHostKeyCheck
	}
	if local.RequestTTY != "" {
		result.RequestTTY = local.RequestTTY
	}
	if local.LocalForward != nil {
		result.LocalForward = local.LocalForward
	}
	if local.RemoteForward != nil {
		result.RemoteForward = local.RemoteForward
	}

	// Merge maps: global as base, local overrides
	result.EnvVars = mergeMaps(global.EnvVars, local.EnvVars)
	result.ExtraOptions = mergeMaps(global.ExtraOptions, local.ExtraOptions)

	return result
}

func mergeMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(map[string]string)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}
