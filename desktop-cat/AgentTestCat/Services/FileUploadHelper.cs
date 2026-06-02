namespace AgentTestCat.Services;

public static class FileUploadHelper
{
    public static IEnumerable<(string EntryName, string FullPath)> CollectUploadEntries(IEnumerable<string> paths)
    {
        foreach (var raw in paths)
        {
            var p = raw.Trim();
            if (string.IsNullOrEmpty(p)) continue;

            if (System.IO.Directory.Exists(p))
            {
                var root = System.IO.Path.GetFullPath(p);
                foreach (var file in System.IO.Directory.EnumerateFiles(root, "*", System.IO.SearchOption.AllDirectories))
                {
                    var rel = System.IO.Path.GetRelativePath(root, file).Replace('\\', '/');
                    yield return (rel, file);
                }
                continue;
            }

            if (System.IO.File.Exists(p))
                yield return (System.IO.Path.GetFileName(p), System.IO.Path.GetFullPath(p));
        }
    }

    public static string GuessKind(string name)
    {
        var n = name.Replace('\\', '/').ToLowerInvariant();
        if (System.Text.RegularExpressions.Regex.IsMatch(n, @"\.(png|jpe?g|gif|webp|bmp)$"))
            return "image";
        if (n.Contains('/'))
            return "folder";
        return "file";
    }
}
