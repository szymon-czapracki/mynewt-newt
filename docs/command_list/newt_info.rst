newt info 
----------

Show information about the current project.

Usage:
^^^^^^

.. code-block:: console

        newt info [flags]

Flags:
^^^^^^

.. code-block:: console

            --json        Print the project info as JSON
            --porcelain   Print the project info as stable, line-based text for scripts
        -r, --remote      Fetch latest repos to determine if upgrades are required

Global Flags:
^^^^^^^^^^^^^

.. code-block:: console

        -h, --help              Help for newt commands
        -j, --jobs int          Number of concurrent build jobs (0 = number of CPUs) (default 8)
        -l, --loglevel string   Log level (default "WARN")
        -o, --outfile string    Filename to tee output to
        -q, --quiet             Be quiet; only display error output
        -s, --silent            Be silent; don't output anything
        -v, --verbose           Enable verbose output when executing commands

Description
^^^^^^^^^^^

Displays the repositories in the current project (the local as well as all the external repositories fetched). It also displays the packages in the local repository.

Output for scripts
^^^^^^^^^^^^^^^^^^

``--porcelain`` and ``--json`` describe the whole project in a format meant
for scripts and other tools rather than people: the repositories, all targets
with their variables and their complete syscfg (every setting of the target's
build, as ``newt target config show`` displays it), and all packages.  Only
the requested format is written to standard output; errors go to standard
error, with a non-zero exit status.  Paths are relative to the project
directory.

Both formats are stable.  New fields may be added, but existing ones are
not renamed or removed without increasing ``format_version``.

``--porcelain`` prints one value per line, in the form
``<kind> <name> <key> <value>``, where ``<kind>`` is ``project``, ``repo``,
``target`` or ``package``.  The first three fields never contain spaces; the
value is the rest of the line, with backslashes and newlines escaped as
``\\`` and ``\n``.  Lines with empty values are left out.  A target's
variables have keys ``var.<name>``, the fields of its syscfg settings
``syscfg.<setting>.<field>``, and the problems that would make the build
fail ``syscfg_error``.  The keys of lists (``restriction``, ``history`` and
``syscfg_error``) are repeated once per element, in order:

.. code-block:: console

    $ newt info --porcelain
    project my_project format_version 1
    project my_project newt_version 1.16.0
    project my_project path /home/me/my_project
    repo apache-mynewt-core path repos/apache-mynewt-core
    repo apache-mynewt-core local false
    repo apache-mynewt-core url https://github.com/apache/mynewt-core.git
    repo apache-mynewt-core requested 1.12.0
    repo apache-mynewt-core installed 1.12.0
    repo apache-mynewt-core commit c81875e23d714c2c09de722a73df0feb6bdbdad4
    repo apache-mynewt-core needs_upgrade false
    target targets/blinky repo my_project
    target targets/blinky path targets/blinky
    target targets/blinky var.app @apache-mynewt-core/apps/blinky
    target targets/blinky var.bsp @apache-mynewt-core/hw/bsp/nordic_pca10056
    target targets/blinky var.build_profile optimized
    target targets/blinky syscfg.LOG_LEVEL.value 1
    target targets/blinky syscfg.LOG_LEVEL.default 0
    target targets/blinky syscfg.LOG_LEVEL.description Limits what level log messages are compiled in.
    target targets/blinky syscfg.LOG_LEVEL.type raw
    target targets/blinky syscfg.LOG_LEVEL.state good
    target targets/blinky syscfg.LOG_LEVEL.package @apache-mynewt-core/sys/log/full
    target targets/blinky syscfg.LOG_LEVEL.history @apache-mynewt-core/sys/log/full 0
    target targets/blinky syscfg.LOG_LEVEL.history targets/blinky 1
    ...
    package @apache-mynewt-core/apps/blinky type app
    package @apache-mynewt-core/apps/blinky repo apache-mynewt-core
    package @apache-mynewt-core/apps/blinky path repos/apache-mynewt-core/apps/blinky
    package @apache-mynewt-core/apps/blinky description Blinks an LED.
    ...

    $ newt info --porcelain | grep '^target .* var.bsp '
    $ newt info --porcelain | grep '^target targets/blinky syscfg\.LOG_LEVEL\.value '

A setting's ``restriction`` is ``notnull``, ``expr <expression>`` or
``range <ranges>``.  Each ``history`` line is ``<package> <value>``, starting
with the package that defines the setting; ``newt`` stands for values newt
itself sets.  ``syscfg_error`` lines tell why ``newt build`` would fail,
e.g. for a target without an app.

``--json`` prints the same information as one JSON object:

.. code-block:: console

    $ newt info --json
    {
      "format_version": 1,
      "newt_version": "1.16.0",
      "project": {"name": "my_project", "path": "/home/me/my_project"},
      "repos": [
        {
          "name": "apache-mynewt-core",
          "path": "repos/apache-mynewt-core",
          "local": false,
          "url": "https://github.com/apache/mynewt-core.git",
          "requested": "1.12.0",
          "installed": "1.12.0",
          "commit": "c81875e23d714c2c09de722a73df0feb6bdbdad4",
          "dirty": "",
          "needs_upgrade": false,
          "error": ""
        }
      ],
      "targets": [
        {
          "name": "targets/blinky",
          "repo": "my_project",
          "path": "targets/blinky",
          "vars": {
            "app": "@apache-mynewt-core/apps/blinky",
            "bsp": "@apache-mynewt-core/hw/bsp/nordic_pca10056",
            "build_profile": "optimized"
          },
          "syscfg": [
            {
              "name": "LOG_LEVEL",
              "value": "1",
              "default": "0",
              "description": "Limits what level log messages are compiled in.",
              "type": "raw",
              "state": "good",
              "package": "@apache-mynewt-core/sys/log/full",
              "value_ref": "",
              "choices": [],
              "restrictions": [],
              "history": [
                {"source": "@apache-mynewt-core/sys/log/full", "value": "0"},
                {"source": "targets/blinky", "value": "1"}
              ]
            }
          ],
          "syscfg_errors": []
        }
      ],
      "packages": [
        {
          "name": "@apache-mynewt-core/apps/blinky",
          "type": "app",
          "repo": "apache-mynewt-core",
          "path": "repos/apache-mynewt-core/apps/blinky",
          "description": "Blinks an LED."
        }
      ]
    }

In JSON, a setting's ``restrictions`` have a ``type`` of ``notnull``,
``expr`` or ``range``, and the expression or ranges in ``expr``, and its
``history`` is a list of ``source`` and ``value`` pairs.

With ``--remote``, the repositories' remotes are queried first, and
``needs_upgrade`` tells whether ``newt upgrade`` would change a repository.
