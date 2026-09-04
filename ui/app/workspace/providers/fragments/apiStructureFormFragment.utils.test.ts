import { describe, expect, it } from "vitest";

import { customProviderConfigSchema, formCustomProviderConfigSchema } from "@/lib/types/schemas";
import { getApiStructureFormDefaults } from "./apiStructureFormFragment.utils";

describe("getApiStructureFormDefaults", () => {
	it("defaults an omitted preservation policy to false", () => {
		expect(getApiStructureFormDefaults({ base_provider_type: "anthropic" }).preserve_mid_conversation_system_messages).toBe(false);
	});

	it("retains an enabled policy and sibling configuration", () => {
		const defaults = getApiStructureFormDefaults({
			base_provider_type: "anthropic",
			preserve_mid_conversation_system_messages: true,
			allowed_requests: {
				text_completion: true,
				text_completion_stream: true,
				chat_completion: false,
				chat_completion_stream: true,
				responses: true,
				responses_stream: true,
				embedding: true,
				speech: true,
				speech_stream: true,
				transcription: true,
				transcription_stream: true,
				image_generation: true,
				image_generation_stream: true,
				image_edit: true,
				image_edit_stream: true,
				image_variation: true,
				count_tokens: true,
				list_models: true,
				rerank: true,
				video_generation: true,
				video_edit: true,
				video_retrieve: true,
				video_download: true,
				video_delete: true,
				video_list: true,
				video_remix: true,
				websocket_responses: true,
				realtime: true,
			},
			request_path_overrides: { responses: "/messages" },
		});

		expect(defaults.preserve_mid_conversation_system_messages).toBe(true);
		expect(defaults.allowed_requests.chat_completion).toBe(false);
		expect(defaults.allowed_requests.responses).toBe(true);
		expect(defaults.request_path_overrides).toEqual({ responses: "/messages" });
	});
});

describe("custom provider preservation policy schemas", () => {
	for (const schema of [customProviderConfigSchema, formCustomProviderConfigSchema]) {
		it("accepts booleans and rejects non-booleans", () => {
			expect(schema.safeParse({ base_provider_type: "anthropic", preserve_mid_conversation_system_messages: true }).success).toBe(true);
			expect(schema.safeParse({ base_provider_type: "anthropic", preserve_mid_conversation_system_messages: "true" }).success).toBe(false);
		});
	}
});