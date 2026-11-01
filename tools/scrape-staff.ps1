# Scrapes DJ/host names, bios and photos from the old Wix site into wix-cache\staff.json and images\staff\.
# Then run build-staff-data.ps1 to regenerate staff-data.js (overwrites hand edits; see CLAUDE.md).
$ErrorActionPreference = "Stop"
$site = Split-Path $PSScriptRoot -Parent
$wix  = Join-Path $PSScriptRoot "wix-cache"   # scraped HTML/JSON lands here
New-Item -ItemType Directory -Force $wix | Out-Null
New-Item -ItemType Directory -Force "$site\images\staff" | Out-Null
Add-Type -AssemblyName System.Drawing
$enc = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object MimeType -eq 'image/jpeg'
$ep = New-Object System.Drawing.Imaging.EncoderParameters 1
$ep.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality, [long]82)

foreach ($p in "team-1","team") { [IO.File]::WriteAllText("$wix\$p.html", (Invoke-WebRequest "https://muddys2010.wixsite.com/muddys/$p" -UseBasicParsing).Content) }
$skip = '7fc3a3a7|f7783238|f7c8f752|d1d8015e|b8b62269'
$out = @{}
foreach ($page in "team-1", "team") {
  $c = [IO.File]::ReadAllText("$wix\$page.html")
  $c = $c.Substring($c.IndexOf("<body"))
  $c = [regex]::Replace($c, '<script[\s\S]*?</script>', '')
  $links = [regex]::Matches($c, "href=`"https://muddys2010\.wixsite\.com/muddys/$page/([^`"#]+)`"") | ForEach-Object { $_.Groups[1].Value } | Select-Object -Unique
  $imgs  = [regex]::Matches($c, 'static\.wixstatic\.com/media/([0-9a-f]{6}_[0-9a-f]{32}~mv2\.(?:png|jpg))') | ForEach-Object { $_.Groups[1].Value } | Where-Object { $_ -notmatch $skip } | Select-Object -Unique
  if ($links.Count -ne $imgs.Count) { throw "$page mismatch: $($links.Count) links vs $($imgs.Count) images" }
  $list = @()
  for ($i = 0; $i -lt $links.Count; $i++) {
    $slug = $links[$i]
    $d = (Invoke-WebRequest "https://muddys2010.wixsite.com/muddys/$page/$slug" -UseBasicParsing).Content
    $d = $d.Substring($d.IndexOf("<body"))
    $d = [regex]::Replace($d, '<script[\s\S]*?</script>|<style[\s\S]*?</style>', '')
    $t = @([regex]::Matches($d, '>([^<>]{1,1200})<') | ForEach-Object { [Net.WebUtility]::HtmlDecode($_.Groups[1].Value).Replace([string][char]0x200B, '').Trim() } | Where-Object { $_ })
    $start = [array]::IndexOf($t, "Use tab to navigate through the menu items.")
    $end = [array]::IndexOf($t, "Follow us on")
    if ($start -lt 0 -or $end -lt 0) {
      # Empty detail page: fall back to a name from the URL, no bio
      $name = (Get-Culture).TextInfo.ToTitleCase(($slug -replace '-', ' '))
      $bio = @()
    } else {
      $body = @($t[($start + 1)..($end - 1)] | Where-Object { $_ -ne '< Back' })
      $name = $body[0]
      $bio = @($body | Select-Object -Skip 1)
    }
    # Photo: download original, crop to centered square, 600px
    $file = "$page-$slug.jpg" -replace '^team-1-', 'dj-' -replace '^team-', 'host-' -replace '^dj-dj-', 'dj-'
    $tmp = "$wix\img-$slug"
    Invoke-WebRequest "https://static.wixstatic.com/media/$($imgs[$i])" -OutFile $tmp -UseBasicParsing
    $src = [System.Drawing.Image]::FromFile($tmp)
    $s = [Math]::Min($src.Width, $src.Height); $sz = [Math]::Min(600, $s)
    $bmp = New-Object System.Drawing.Bitmap $sz, $sz
    $g = [System.Drawing.Graphics]::FromImage($bmp); $g.InterpolationMode = 'HighQualityBicubic'
    $g.DrawImage($src, (New-Object System.Drawing.Rectangle 0, 0, $sz, $sz), (New-Object System.Drawing.Rectangle ([int](($src.Width - $s) / 2)), 0, $s, $s), 'Pixel')
    $bmp.Save("$site\images\staff\$file", $enc, $ep); $g.Dispose(); $bmp.Dispose(); $src.Dispose()
    $list += [ordered]@{ name = $name; photo = "images/staff/$file"; bio = $bio }
    Write-Output "$page  $name  ($($bio.Count) bio lines, src ${s}px)"
  }
  $out[$page] = $list
}
$json = [ordered]@{ djs = $out["team-1"]; hosts = $out["team"] } | ConvertTo-Json -Depth 5
[IO.File]::WriteAllText("$wix\staff.json", $json, (New-Object System.Text.UTF8Encoding $false))
