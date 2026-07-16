package forwarder

import (
	"strings"

	"cursor/gen/agentv1"
	"cursor/internal/skillsconfig"
)

var configuredSkillsResolver = skillsconfig.NewResolver()

func mergeConfiguredSkillDescriptors(requestContext *agentv1.RequestContext, descriptors []*agentv1.SkillDescriptor) []*agentv1.SkillDescriptor {
	if requestContext == nil || configuredSkillsResolver == nil {
		return descriptors
	}

	workspaceRoot := firstNonEmpty(
		strings.TrimSpace(requestContext.GetEnv().GetProjectFolder()),
		firstWorkspacePath(requestContext.GetEnv().GetWorkspacePaths()),
	)
	resolved, err := configuredSkillsResolver.Resolve(workspaceRoot)
	if err != nil || len(resolved) == 0 {
		return descriptors
	}

	orderedKeys := make([]string, 0, len(descriptors)+len(resolved))
	descriptorsByKey := make(map[string]*agentv1.SkillDescriptor, len(descriptors)+len(resolved))
	addDescriptor := func(descriptor *agentv1.SkillDescriptor) {
		normalized := normalizeSkillDescriptor(descriptor)
		if normalized == nil {
			return
		}
		key := skillDescriptorKey(normalized)
		if key == "" {
			return
		}
		if existing, ok := descriptorsByKey[key]; ok {
			mergeSkillDescriptor(existing, normalized)
			return
		}
		orderedKeys = append(orderedKeys, key)
		descriptorsByKey[key] = normalized
	}

	for _, descriptor := range descriptors {
		addDescriptor(descriptor)
	}
	for _, skill := range resolved {
		addDescriptor(configuredSkillToDescriptor(skill))
	}

	merged := make([]*agentv1.SkillDescriptor, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		if descriptor := descriptorsByKey[key]; descriptor != nil {
			merged = append(merged, descriptor)
		}
	}
	return merged
}

func configuredSkillToDescriptor(skill skillsconfig.ResolvedSkill) *agentv1.SkillDescriptor {
	return &agentv1.SkillDescriptor{
		Name:           strings.TrimSpace(skill.Name),
		Description:    strings.TrimSpace(skill.Description),
		FolderPath:     strings.TrimSpace(skill.FolderPath),
		Enabled:        true,
		ReadmeFilePath: strings.TrimSpace(skill.ReadmePath),
		PackageType:    configuredSkillPackageType(skill),
	}
}

func configuredSkillPackageType(skill skillsconfig.ResolvedSkill) agentv1.PackageType {
	switch skill.Scope {
	case skillsconfig.ScopeProject:
		return agentv1.PackageType_PACKAGE_TYPE_CURSOR_PROJECT
	default:
		return agentv1.PackageType_PACKAGE_TYPE_CURSOR_PERSONAL
	}
}