#!/usr/bin/env python3
"""Refresh the checked-in, reviewed LiteLLM snapshot used on first launch."""
import datetime
import json
from pathlib import Path
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
URL = 'https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json'
FIELDS = {
    'max_input_tokens': 'maxInputTokens',
    'input_cost_per_token': 'inputCostPerToken',
    'output_cost_per_token': 'outputCostPerToken',
    'cache_read_input_token_cost': 'cacheReadInputTokenCost',
    'cache_creation_input_token_cost': 'cacheCreationInputTokenCost',
    'input_cost_per_token_above_200k_tokens': 'inputCostPerTokenAbove200kTokens',
    'output_cost_per_token_above_200k_tokens': 'outputCostPerTokenAbove200kTokens',
    'cache_read_input_token_cost_above_200k_tokens': 'cacheReadInputTokenCostAbove200kTokens',
    'cache_creation_input_token_cost_above_200k_tokens': 'cacheCreationInputTokenCostAbove200kTokens',
}


def normalize(raw):
    import math
    models = {}
    for name, entry in raw.items():
        if not isinstance(entry, dict):
            continue
        rates = {target: entry[source] for source, target in FIELDS.items() if source in entry}
        provider = entry.get('provider_specific_entry')
        if isinstance(provider, dict) and 'fast' in provider:
            rates['fastMultiplier'] = provider['fast']
        if 'maxInputTokens' in rates and (not isinstance(rates['maxInputTokens'], int) or isinstance(rates['maxInputTokens'], bool)):
            rates.pop('maxInputTokens')
        if 'inputCostPerToken' not in rates or 'outputCostPerToken' not in rates:
            continue
        if any(isinstance(v, bool) or not isinstance(v, (int, float)) or not math.isfinite(v) or v < 0 for v in rates.values()):
            continue
        models[name] = rates
    if not models:
        raise ValueError('No valid token prices in catalog')
    return models


if __name__ == '__main__':
    with urllib.request.urlopen(URL, timeout=20) as response:
        raw = response.read(32 * 1024 * 1024 + 1)
    if len(raw) > 32 * 1024 * 1024:
        raise ValueError('Catalog exceeds size limit')
    data = {'version': 1, 'fetched': datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z'), 'source': URL, 'models': normalize(json.loads(raw))}
    (ROOT / 'internal/app/pricedata/litellm.json').write_text(json.dumps(data, sort_keys=True, separators=(',', ':')) + '\n')
    print(f'Updated {len(data["models"])} model prices. Review the diff before committing.')
