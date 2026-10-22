Auto-Compose
============
The nchat bundled default auto-compose utility `compose` uses external
services for chat completion, and generally requires an API key
(set in environment) to work. Environment variables to set:

    OpenAI: OPENAI_API_KEY
    Gemini: GEMINI_API_KEY


Privacy and invocation
----------------------
`auto_compose_enabled=0` is the default. While disabled, nchat does not invoke
this helper or automatically upload history for AI replies. Enabling it allows
the auto-compose key (default Alt-I) to invoke the configured command. nchat
writes a local plaintext temporary history file containing sender display names,
your name and up to 25 recent messages by default (controlled by
`auto_compose_history_count`, starting at the selected/latest message). The
bundled script includes those names and message text in the provider payload;
the result is placed in the message entry for you to send. Running `src/compose`
standalone also sends its input file to the chosen service.

OpenAI is the default (`gpt-4o-mini`, `OPENAI_API_KEY`); Gemini uses
`gemini-2.0-flash` and `GEMINI_API_KEY`. Ollama (`-s ollama`) targets
`http://localhost:11434` with the local model name `llama3.2:3b`; install that
model locally first (for example `ollama pull llama3.2:3b`). Ollama is not always
local: choosing a cloud model, forwarding the local server, or selecting a custom
remote endpoint can send the same data remotely. Custom compatible URLs and
`-m` model overrides are your responsibility to review. Verbose mode prints the
request payload, including chat content, to stderr. See [Privacy](PRIVACY.md)
for local storage, telemetry and backup considerations.


Basic Testing
-------------
An [example chat history file](/doc/example-history.txt) is provided for
testing chat completion standalone. Example:

    ./src/compose -c doc/example-history.txt


Testing Services / Models
-------------------------
The utility `compose` may be used with OpenAI-compatible services.

Example usage with default service (OpenAI) and default model (gpt-4o-mini):

    ./src/compose -c doc/example-history.txt

Example usage with OpenAI:

    ./src/compose -s openai -c doc/example-history.txt

Example usage with OpenAI and custom model and longer timeout of 60 secs:

    ./src/compose -s openai -m gpt-5-nano -T 60 -c doc/example-history.txt

Example usage with Google Gemini:

    ./src/compose -s gemini -c doc/example-history.txt

Example usage with Google Gemini and custom model:

    ./src/compose -s gemini -m gemini-2.5-flash -c doc/example-history.txt

Example usage with llama.cpp:

    ./src/compose -s "http://192.168.10.159:8080" -c doc/example-history.txt

Example starting llama.cpp server:

    llama-server --port 8080 -m ./models/llama-2-13b-chat.Q4_K_M.gguf


Configuring Custom Service / Model
----------------------------------
Edit `ui.conf` to match the desired compose path and usage.

The `compose` script is bundled inside the nchat executable and extracted to
a temporary location at runtime, so there is no installed copy to reference.
To use custom arguments, copy [compose](/src/compose) from the source tree
(or download it) to a location of your choice, e.g:

    cp src/compose ~/.local/bin/compose

Example usage with Google Gemini and custom model:

    auto_compose_command=python3 ~/.local/bin/compose -s gemini -m gemini-2.0-flash -c '%1'

Example usage with OpenAI and custom model and longer timeout of 60 secs:

    auto_compose_command=python3 ~/.local/bin/compose -s openai -m gpt-5-nano -T 60 -c '%1'

Example usage with custom prompt and max token limit of 100:

    auto_compose_command=python3 ~/.local/bin/compose -p "Suggest {your_name}'s next reply in a joking manner." -M 100 -c '%1'

