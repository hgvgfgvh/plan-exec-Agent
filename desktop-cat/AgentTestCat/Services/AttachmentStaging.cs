namespace AgentTestCat.Services;

public sealed class PendingAttachment
{
    public string Id { get; set; } = Guid.NewGuid().ToString("N");
    public string DisplayName { get; set; } = "";
    public string Kind { get; set; } = "file";
    /// <summary>本地源路径，用于发送前删除后重新 staging。</summary>
    public string LocalPath { get; set; } = "";
}

public sealed class AttachmentStaging
{
    public string? StagingId { get; private set; }
    public List<PendingAttachment> Items { get; } = new();

    public bool HasPending => Items.Count > 0;

    public void SetUploaded(string stagingId, IEnumerable<PendingAttachment> items)
    {
        StagingId = stagingId;
        Items.Clear();
        Items.AddRange(items);
    }

    public void AppendUploaded(string stagingId, IEnumerable<PendingAttachment> items)
    {
        StagingId = stagingId;
        Items.AddRange(items);
    }

    public void Clear()
    {
        StagingId = null;
        Items.Clear();
    }
}

public sealed class UploadFileEntry
{
    public string Name { get; set; } = "";
    public string? Kind { get; set; }
}

public sealed class UploadResult
{
    public string StagingId { get; set; } = "";
    public List<UploadFileEntry> Files { get; set; } = new();
    public List<string> Skipped { get; set; } = new();
}
