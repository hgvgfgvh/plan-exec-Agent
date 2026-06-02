namespace AgentTestCat.Models;

public sealed class ChatMessage
{
    public string Id { get; set; } = Guid.NewGuid().ToString("N");
    public string Source { get; set; } = "";
    public string Text { get; set; } = "";
    public bool IsUser { get; set; }
    public bool IsStreaming { get; set; }
    public DateTime At { get; set; } = DateTime.Now;
}
