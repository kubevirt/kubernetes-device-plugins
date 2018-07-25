import subprocess


class CommandExecutionError(Exception):
    pass


def ssh(node, command):
    p = subprocess.Popen(
        ['docker', 'exec', 'kubevirt-' + node, 'ssh.sh'] + command,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    stdout, stderr = p.communicate()

    if p.returncode != 0:
        raise CommandExecutionError(p.returncode, stderr.split('\n')[2:-1])

    return stdout.split('\n')[:-1]
