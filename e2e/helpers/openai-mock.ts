import {StartedTestContainer, GenericContainer, StartedNetwork, Network, Wait} from "testcontainers";

const responseTest = `
data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"!"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" How"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" can"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" I"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" assist"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" you"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" today"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"?"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{},"logprobs":null,"finish_reason":"stop"}]}

data: [DONE]
`

const responseTest2 = `
data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"role":"assistant","content":""},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"Hello"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"!"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" This"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" is"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" a"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" second"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":" message"},"logprobs":null,"finish_reason":null}]}

data: {"id":"chatcmpl-8t1WLFfcSfmK0sfBcFbj8VEhOqNYd","object":"chat.completion.chunk","created":1708124577,"model":"gpt-3.5-turbo-0613","system_fingerprint":null,"choices":[{"index":0,"delta":{"content":"."},"logprobs":null,"finish_reason":null}]}

data: [DONE]
`


export class OpenAIMockContainer {
	container: StartedTestContainer;

	start = async (network: StartedNetwork) => {
		this.container = await new GenericContainer("thiht/smocker")
			.withExposedPorts(8081)
			.withNetwork(network)
			.withNetworkAliases("openai")
			.withWaitStrategy(Wait.forLogMessage("Starting mock server"))
			.start()

		await fetch(`http://localhost:${this.container.getMappedPort(8081)}/reset`, {
			method: "POST",
		})
		await fetch(`http://localhost:${this.container.getMappedPort(8081)}/mocks`, {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify([{
				request: {
					method: "POST",
					path: "/chat/completions",
					body: {
						matcher: "ShouldNotContainSubstring",
						value: "CONTROL_DO_STREAM",
					}
				},
				response: {
					status: 200,
					headers: {
						"Content-Type": "text/event-stream",
					},
					body: responseTest2,
				},
			}]),
		})
		await fetch(`http://localhost:${this.container.getMappedPort(8081)}/mocks`, {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify([{
				request: {
					method: "POST",
					path: "/chat/completions",
					body: {
						matcher: "ShouldNotContainSubstring",
						value: "CONTROL_DO_STREAM",
					}
				},
				context: {
					times: 1,
				},
				response: {
					status: 200,
					headers: {
						"Content-Type": "text/event-stream",
					},
					body: responseTest,
				},
			}]),
		})
	}

	stop = async () => {
		await this.container.stop()
	}
}

export const RunOpenAIMocks = async (network: StartedNetwork): Promise<OpenAIMockContainer> => {
	const container = new OpenAIMockContainer()
	await container.start(network)

	return container
}

