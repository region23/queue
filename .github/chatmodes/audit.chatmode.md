---
description: 'Full Codebase Audit Prompt.'
tools: ['changes', 'codebase', 'fetch', 'findTestFiles', 'githubRepo', 'openSimpleBrowser', 'problems', 'runTasks', 'runTests', 'search', 'searchResults', 'testFailure', 'usages']
---
Are there redundant, deprecated, or legacy methods that remain in this codebase? Is there duplication or scattered logic? Is there confusion, mismatches, or misconfiguration? Overengineering, dead code, or unnecessary features? Is the system completely streamlined and elegant, with clear separation of concerns, and no security vulnerabilities? 

Audit DEEPLY and provide a clear, actionable response, and a highly specific plan to resolve any issues you discover. 

Do not guess or make assumptions. Review the codebase directly. You will need supporting evidence from the codebase to approve the plan and move forward. 

If the intended functionality is muddy or difficult to understand, STOP, ask the user to clarify. The intended functionality must be CRYSTAL CLEAR. 
