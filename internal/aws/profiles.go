package aws

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultProfileName = "default"
)

type SharedConfigPaths struct {
	ConfigFile      string
	CredentialsFile string
}

func DefaultSharedConfigPaths() SharedConfigPaths {
	home, _ := os.UserHomeDir()
	return SharedConfigPaths{
		ConfigFile:      filepath.Join(home, ".aws", "config"),
		CredentialsFile: filepath.Join(home, ".aws", "credentials"),
	}
}

func LoadProfiles(paths SharedConfigPaths) ([]Profile, error) {
	sections, err := parseINIFile(paths.ConfigFile, true)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parse aws config file: %w", err)
	}

	credentialsSections, err := parseINIFile(paths.CredentialsFile, false)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parse aws credentials file: %w", err)
	}

	merged := map[string]Profile{}
	for name, props := range credentialsSections {
		merged[name] = newProfile(name, props)
	}

	for name, props := range sections {
		current, ok := merged[name]
		if !ok {
			merged[name] = newProfile(name, props)
			continue
		}

		if current.Properties == nil {
			current.Properties = map[string]string{}
		}
		for k, v := range props {
			current.Properties[k] = v
		}
		current.Region = current.Properties["region"]
		current.Kind = ClassifyProfile(current)
		merged[name] = current
	}

	profiles := make([]Profile, 0, len(merged))
	for _, p := range merged {
		if p.Name == "" {
			continue
		}
		if p.Kind == "" {
			p.Kind = ClassifyProfile(p)
		}
		profiles = append(profiles, p)
	}

	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Name == defaultProfileName {
			return true
		}
		if profiles[j].Name == defaultProfileName {
			return false
		}
		return profiles[i].Name < profiles[j].Name
	})

	return profiles, nil
}

func ClassifyProfile(profile Profile) ProfileKind {
	props := profile.Properties
	switch {
	case props == nil:
		return ProfileKindUnknown
	case props["login_session"] != "":
		return ProfileKindLoginSession
	case props["credential_process"] != "":
		return ProfileKindCredentialProcess
	case props["role_arn"] != "":
		return ProfileKindAssumeRole
	case props["sso_session"] != "" || props["sso_start_url"] != "":
		return ProfileKindSSO
	case props["aws_access_key_id"] != "" || props["credential_source"] != "" || props["source_profile"] != "":
		return ProfileKindStandard
	default:
		return ProfileKindStandard
	}
}

func FindProfile(profiles []Profile, name string) (Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return Profile{}, false
}

func parseINIFile(path string, awsConfigStyle bool) (map[string]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sections := map[string]map[string]string{}
	var current string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			if awsConfigStyle && strings.HasPrefix(name, "profile ") {
				name = strings.TrimSpace(strings.TrimPrefix(name, "profile "))
			}
			current = name
			if _, ok := sections[current]; !ok {
				sections[current] = map[string]string{}
			}
			continue
		}

		if current == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		sections[current][key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return sections, nil
}

func newProfile(name string, props map[string]string) Profile {
	copyProps := map[string]string{}
	for k, v := range props {
		copyProps[k] = v
	}
	profile := Profile{
		Name:       name,
		Properties: copyProps,
		Region:     copyProps["region"],
	}
	profile.Kind = ClassifyProfile(profile)
	return profile
}
