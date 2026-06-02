namespace AgentTestCat.Services;

public static class CatQuips
{
    private static readonly string[] Poke =
    [
        "喵？", "干嘛～", "别戳啦", "我在呢", "有事说嘛", "摸头可以，别乱戳",
        "哼！", "再戳咬你哦（假的）"
    ];

    private static readonly string[] PokeSpam =
    [
        "好啦好啦！", "知道你在啦", "戳够了吗～", "我要去睡觉了"
    ];

    public static string RandomPoke(bool spam) =>
        spam ? PokeSpam[Random.Shared.Next(PokeSpam.Length)] : Poke[Random.Shared.Next(Poke.Length)];
}
