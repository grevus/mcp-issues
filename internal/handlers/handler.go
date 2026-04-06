package handlers

import "context"

// Handler — generic функция-обработчик MCP tool. Не знает про MCP SDK и Echo.
// Адаптер в internal/register оборачивает Handler в SDK-совместимую сигнатуру.
type Handler[In, Out any] func(ctx context.Context, in In) (Out, error)
