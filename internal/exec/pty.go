package exec

// PTY support is intentionally left minimal in the MVP because the final
// interactive shell session is delegated to the AWS CLI and Session Manager
// plugin, which manage TTY behavior cross-platform.
