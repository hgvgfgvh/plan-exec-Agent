using System;
using System.Windows.Media.Imaging;
using System.Windows.Media;

var uri = new Uri(@"C:\DATA\GODATA\AgentTestPCAPPCat\AgentTestCat\Assets\cat\cat_idle.png");
var dec = BitmapDecoder.Create(uri, BitmapCreateOptions.PreservePixelFormat, BitmapCacheOption.OnLoad);
var frame = dec.Frames[0];
Console.WriteLine("Format: " + frame.Format);
var conv = new FormatConvertedBitmap(frame, PixelFormats.Bgra32, null, 0);
conv.Freeze();
int w = conv.PixelWidth, h = conv.PixelHeight, stride = w * 4;
var px = new byte[h * stride];
conv.CopyPixels(px, stride, 0);
void Sample(int x,int y){ int p=(y*w+x)*4; Console.WriteLine($"{x},{y}: B={px[p]} G={px[p+1]} R={px[p+2]} A={px[p+3]}"); }
Sample(512,512); Sample(512,600); Sample(0,0); Sample(100,100);
