# Orchid

Orchid is a command-line application orchestrator designed to manage the lifecycle of on-premises applications with complex inter dependencies. It provides a simple interface to bring up and bring down application suites leveraging SSH for remote operations in a specified environment, ensuring proper sequencing, monitoring, and rollback capabilities.

Table of Contents

Features
Prerequisites
Installation
Configuration
Usage
Commands
Examples
Logging
Testing
Contributing
License

### Features
---
- **Application Lifecycle Management**: Start and stop applications in the correct order based on dependencies.
- **Monitoring**: Continuously monitor applications during the startup process to detect failures.
- **Rollback Mechanism**: Automatically rollback started applications if a failure occurs during the bring-up process.
- **Concurrency Control**: Prevent multiple operations from running simultaneously using file-based locking.
- **SSH Integration**: Execute commands on remote hosts via SSH, supporting key-based authentication.
- **Configurable Environments**: Manage multiple environments with different configurations.
- **Extensible**: Easily add new applications or environments by updating the configuration file.

### Prerequisites
---
Go: Ensure you have Go installed (version 1.21 or later). You can download it from golang.org.
Installation

#### Build the Application
``` bash
go build -o orchid .
```
This command compiles the Orchid CLI and produces an executable named orchid in your current directory.

#### Install Dependencies
Orchid uses several Go packages. Ensure all dependencies are installed by running:

```bash
go mod tidy
```

### Configuration
---
Orchid uses a YAML configuration file to define environments and applications. Below is an example of a valid configuration.

Example config.yaml
```yaml
environments:
  dev:
    remote_user: deploy
    applications:
      - name: app1
        host: dev-host1.example.com
        start_command: "./start_app1.sh"
        stop_command: "./stop_app1.sh"
        check_command: "./check_app1.sh"
        check_interval: 5
      - name: app2
        host: dev-host2.example.com
        start_command: "./start_app2.sh"
        stop_command: "./stop_app2.sh"
        check_command: "./check_app2.sh"
        check_interval: 5
  staging:
    remote_user: deploy
    applications:
      - name: app3
        host: staging-host1.example.com
        start_command: "./start_app3.sh"
        stop_command: "./stop_app3.sh"
        check_command: "./check_app3.sh"
        check_interval: 5
```

#### Configuration Fields
- **environments**: A map of environment names to their configurations.
- **remote_user**: The SSH user used to connect to the hosts.
- **applications**: A list of applications managed in the environment.
  - **name**: The name of the application.
  - **host**: The hostname or IP address where the application runs.
  - **start_command**: The command to start the application.
  - **stop_command**: The command to stop the application.
  - **check_command**: The command to verify if the application is running.
  - **check_interval**: The interval (in seconds) between status checks.

### Usage
--- 
```bash
orchid [command] [flags]
```
Brings up all applications in the specified environment.
``` bash
orchid up --env <environment> --ssh-key <path_to_ssh_key>
```

Brings down all applications in the specified environment.
``` bash
orchid down --env <environment> --ssh-key <path_to_ssh_key>
```

Example Config File
Create a file named orchid.yml in the same directory as the orchid executable.

```yaml
environments:
  staging:
    remote_user: "staging_user"
    applications:
      - name: "redis"
        host: "staging-redis.example.com"
        start_command: "systemctl start redis"
        stop_command: "systemctl stop redis"
        check_command: "systemctl is-active redis"
        check_interval: 5
      - name: "api"
        host: "staging-api.example.com"
        start_command: "systemctl start api"
        stop_command: "systemctl stop api"
        check_command: "systemctl is-active api"
        check_interval: 5
      - name: "web"
        host: "staging-web.example.com"
        start_command: "systemctl start web"
        stop_command: "systemctl stop web"
        check_command: "systemctl is-active web"
        check_interval: 5
```
### Running the Application
1. Bring Up Applications
```bash
orchid up --env staging --ssh-key ~/.ssh/id_rsa
```
- This command will:
  - Acquire a lock to ensure no other operations are running.
  - Start the applications in the order they are defined.
  - Monitor each application during startup.
  - If any application fails during startup, it will rollback the started applications.

2. Bring Down Applications
```bash
orchid down --env staging --ssh-key ~/.ssh/id_rsa
```
- This command will:
  - Acquire a lock to ensure no other operations are running.
  - Stop the applications in reverse order.
  - Ensure each application has stopped successfully.

#### Logging
Logs are written to standard output.
The log level can be adjusted using the --log-level flag.
Example:
```bash
orchid up --env production --ssh-key ~/.ssh/id_rsa --log-level DEBUG
``` 

#### Flags and Environment Variables
- Configuration File
  - -c, --config: Specify the path to the configuration file.
  - Environment Variable: ORCHID_CONFIG_PATH
- Environment
  - -e, --env: Specify the environment to operate on.
  - Environment Variable: ORCHID_ENV
- SSH Key
  - -k, --ssh-key: Specify the path to the SSH private key.
  - Environment Variable: ORCHID_SSH_KEY_PATH
- Log Level
  - -l, --log-level: Set the log verbosity level.
  - Environment Variable: ORCHID_LOG_LEVEL
- Known Hosts
  - --known-hosts: Specify the path to the known_hosts file (default: ~/.ssh/known_hosts).
  - Environment Variable: ORCHID_KNOWN_HOSTS_PATH
- SSH Port
  - --ssh-port: Specify the SSH port (default: 22).
  - Environment Variable: ORCHID_SSH_PORT

### How It Works

1.  Lock Acquisition
  - Before performing any operation, Orchid acquires a lock to prevent concurrent executions.
  - The lock is implemented using a combination of file locks and exclusive file creation to ensure both cross-process and in-process safety.
2. Application Start-Up (up Command)
  - Applications are started in the order they are defined in the configuration.
  - For each application:
    - It checks if the application is already running.
    - If running, it attempts to stop it before starting.
    - Executes the start command.
    - Waits for the specified check_interval.
    - Runs the check command to verify if the application started successfully.
  - Monitoring During Start-Up:
    - A monitoring goroutine continuously checks the health of started applications.
    - If any application fails during start-up, the operation is canceled, and a rollback is initiated.
  - Rollback:
    - Stops all started applications in reverse order.
3. Application Shutdown (down Command)
  - Applications are stopped in reverse order.
  - For each application:
    - Executes the stop command.
    - Runs the check command to ensure the application has stopped.
4. Error Handling
  - Errors are logged and bubbled up to the user.
  - In case of failures, Orchid ensures resources are cleaned up, and locks are released.

### Development
Running Tests
``` bash
go test ./...
``` 

### Project Structure
- cmd/: Contains the CLI commands (up, down, root).
- internal/:
  - config/: Configuration parsing and validation.
  - orchestrator/: Core logic for managing application lifecycles.
  - ssh/: SSH client and factory implementations.
- logger/: Logging setup and initialization.
