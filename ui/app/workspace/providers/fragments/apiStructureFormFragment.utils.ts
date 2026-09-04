import { CustomProviderConfig } from "@/lib/types/config";

export function getApiStructureFormDefaults(config?: CustomProviderConfig) {
	return {
		base_provider_type: config?.base_provider_type ?? "openai",
		is_key_less: config?.is_key_less ?? false,
		preserve_mid_conversation_system_messages: config?.preserve_mid_conversation_system_messages ?? false,
		allowed_requests: {
			...config?.allowed_requests,
			text_completion: config?.allowed_requests?.text_completion ?? true,
			text_completion_stream: config?.allowed_requests?.text_completion_stream ?? true,
			chat_completion: config?.allowed_requests?.chat_completion ?? true,
			chat_completion_stream: config?.allowed_requests?.chat_completion_stream ?? true,
			responses: config?.allowed_requests?.responses ?? true,
			responses_stream: config?.allowed_requests?.responses_stream ?? true,
			responses_retrieve: config?.allowed_requests?.responses_retrieve ?? false,
			responses_delete: config?.allowed_requests?.responses_delete ?? false,
			responses_cancel: config?.allowed_requests?.responses_cancel ?? false,
			responses_input_items: config?.allowed_requests?.responses_input_items ?? false,
			embedding: config?.allowed_requests?.embedding ?? true,
			speech: config?.allowed_requests?.speech ?? true,
			speech_stream: config?.allowed_requests?.speech_stream ?? true,
			transcription: config?.allowed_requests?.transcription ?? true,
			transcription_stream: config?.allowed_requests?.transcription_stream ?? true,
			count_tokens: config?.allowed_requests?.count_tokens ?? true,
			list_models: config?.allowed_requests?.list_models ?? true,
			ocr: config?.allowed_requests?.ocr ?? true,
			ocr_stream: config?.allowed_requests?.ocr_stream ?? true,
		},
		request_path_overrides: config?.request_path_overrides,
	};
}