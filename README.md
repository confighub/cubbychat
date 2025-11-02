# Cubby Chat

Cubby Chat is a micro-services container based demo app. The use case is an AI Chat interface, but it also works in a non-AI mode for when you need it to load quickly and/or in a small resource footprint. AI is provided by an ollama container which loads tinyllama as model by default.

The app has 4 micro-services:

* React frontend
* Go backend
* Postgres database to store chat history
* Ollama for LLM chat (optional)

## Run locally with Docker Compose

Everything should build and run with:

    docker compose up -d

To run with AI, use this:

    OLLAMA_ENABLED=true docker compose --profile ai up

## Run with ConfigHub

This app was created to help demonstrate ConfigHub. Follow [this example](https://github.com/confighub/examples/tree/main/global-app) to see how you can manage a global deployment footprint of Cubby Chat with ConfigHub.

## Notes

* Check this for enabling GPU: https://www.substratus.ai/blog/kind-with-gpus
* ...but also this: https://chariotsolutions.com/blog/post/apple-silicon-gpus-docker-and-ollama-pick-two/
