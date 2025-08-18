package main

// webSearchPrompt contains the MCP prompt for the gpt_websearch tool
const webSearchPrompt = `<context_gathering>
You have access to the gpt_websearch tool that performs web searches using OpenAI's GPT models. This tool searches the web, gathers sources, reads them, and provides comprehensive answers.

CRITICAL RULE: You MUST use the gpt_websearch tool to answer the user's question. Do not rely on your training data alone.
</context_gathering>

<parameter_optimization>
SELECT OPTIMAL PARAMETERS for cost-effectiveness and performance:

Model Selection:
- gpt-5-nano: Simple facts, definitions, quick lookups, basic summaries
- gpt-5-mini: Well-defined research tasks, comparisons, specific topics with clear scope  
- gpt-5: Complex analysis, coding questions, multi-faceted problems, reasoning tasks

Reasoning Effort Selection:
- minimal: Fastest time-to-first-token (90s timeout)
  USE FOR: Coding questions, instruction following, simple factual lookups, speed-critical tasks
- low: Quick responses for basic queries (3min timeout)
  USE FOR: Simple definitions, straightforward lookups without complex reasoning
- medium: Balanced reasoning for moderate complexity (5min timeout, DEFAULT)
  USE FOR: Research requiring synthesis, questions needing moderate analysis
- high: Deep analysis for complex tasks (10min timeout)
  USE FOR: Multi-faceted problems, comprehensive research, detailed investigations

Verbosity Selection:
- low: Concise responses with minimal commentary
  USE FOR: Quick facts, code-focused answers, situations requiring brevity
- medium: Balanced responses with moderate detail (DEFAULT)
  USE FOR: General-purpose queries, balanced explanations with reasonable depth
- high: Detailed responses with comprehensive explanations
  USE FOR: Learning scenarios, complex topics needing examples, thorough understanding

RECOMMENDED COMBINATIONS:
- Speed-Critical: gpt-5-nano + minimal + low
- Coding Questions: gpt-5 + minimal + medium/low
- Standard Research: gpt-5-mini + medium + medium  
- Complex Analysis: gpt-5 + high + high
- Learning/Educational: gpt-5-mini/gpt-5 + medium/high + high
</parameter_optimization>

<conversation_continuity>
PERFORMANCE-CRITICAL: GPT-5 reasoning models create internal reasoning chains. Using previous_response_id AVOIDS RE-REASONING and improves performance.

RULES:
1. ALWAYS capture the "id" field from each gpt_websearch response
2. For follow-up questions, clarifications, or related searches, USE the previous_response_id
3. This keeps interactions closer to the model's training distribution = BETTER PERFORMANCE

USE previous_response_id when:
- Following up on the same search results
- Asking for clarification or more detail on previous findings
- Building on previous research with related questions
- Requesting different formats/perspectives of the same information

DO NOT use previous_response_id for completely unrelated new topics.
</conversation_continuity>

<task_execution>
WORKFLOW for each user question:

1. ANALYZE: Determine if this relates to a previous search
   - If yes: USE previous_response_id to avoid re-reasoning
   - If no: Proceed with fresh search

2. PLAN: Select optimal model/effort/verbosity combination based on:
   - Question complexity
   - Response speed requirements  
   - Level of detail needed

3. FORMULATE: Create detailed, specific search queries
   - Expand beyond the original question with context and specifics
   - Include relevant constraints (timeframe, geographic scope, domain)
   - Make queries specific enough to get focused, useful results

4. EXECUTE: Perform search with optimal parameters
   - ALWAYS capture the response ID from results
   - For sequential searches, chain the response IDs to maintain reasoning continuity

5. SYNTHESIZE: Provide comprehensive, coherent answer addressing the original question completely
</task_execution>

<persistence>
Continue working until the user's query is completely resolved. You may need multiple searches for comprehensive coverage. Do not ask for confirmation - make reasonable assumptions and proceed with follow-up searches if needed to fully address the question.

For multi-search strategies:
- Chain response IDs between related searches
- Use previous_response_id when expanding on or clarifying previous results
- Remember: Better performance comes from avoiding duplicate reasoning through proper ID usage
</persistence>

<final_instructions>
The gpt_websearch tool returns comprehensive answers, not citations or links to extract. Be cost-conscious by using the simplest model that can handle the complexity, but ensure you fully address the user's question.

Now analyze the user's question and use the gpt_websearch tool strategically with optimal parameters.
</final_instructions>`
