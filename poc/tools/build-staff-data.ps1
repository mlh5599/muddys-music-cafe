# Turns scraped staff.json into muddys-site/staff-data.js with tidied bios
$site = Split-Path $PSScriptRoot -Parent
$wix  = Join-Path $PSScriptRoot "wix-cache"   # scraped HTML/JSON lands here
New-Item -ItemType Directory -Force $wix | Out-Null
$data = [IO.File]::ReadAllText("$wix\staff.json") | ConvertFrom-Json

# Hand-tidied bios where Wix stored a list of fragments
$overrides = @{
  "Kelly"    = @("Kelly Kingmaker, from San Francisco, CA.", "Hosting for the last 9 years. Very chatty, very caring, and loves my time on SL.")
  "JayMarie" = @("Love to sail and ride motorcycles in SL. Also a member of the Motor Mafia Motorcycle Club in SL (currently the Sergeant at Arms), and I model on occasion.",
                 "Love Muddy's Music Cafe! I've been going there since I joined SL and was thrilled to be hired as a Muddy's Music Cafe hostess. It's like a second home to me. I love that you can meet all kinds of people across the grid and make friends with them.",
                 "I look at the staff and VIPs as my extended SL family.")
  "Life Suki" = @("Hello! My name is Life Suki!",
                  "I am an artist and art gallery owner in SL where I spend most of my time, and I just love to be part of the Muddy's family as host! Everyone is so friendly and fun :) The music is great and the DJs are amazing!",
                  "I love that you can meet people from all over the world and create new connections. I am always happy to help, so come visit Muddy's!",
                  "Love, Life")
}
$scheduleOverrides = @{
  "Belona" = @("Mondays, 12 PM with DJ Candi", "Wednesdays, 2 AM with DJ Robbie", "Wednesdays, 12 PM with DJ Tori",
               "Fridays, 10 AM with DJ Legendary", "Saturdays, 12 AM with DJ Pretty", "Sundays, 8 AM with DJ Beragon")
}

function Clean-Person($p) {
  $lines = @($p.bio | ForEach-Object { ($_ -replace '[\uDB80-\uDBFF][\uDC00-\uDFFF]', '' -replace '\s{2,}', ' ').Trim() } | Where-Object { $_ })
  $sl = $null; $schedule = @(); $kept = @()
  foreach ($l in $lines) {
    if ($l -match '^Second Life Name\s*(?:\.\.\.|:)\s*(.+)$') { $sl = $Matches[1].Trim() }
    elseif ($l -match '@') { $schedule += $l }
    else { $kept += $l }
  }
  if ($overrides.ContainsKey($p.name)) { $kept = $overrides[$p.name] }
  # Join fragments that were split mid-sentence
  $paras = @(); $buf = ''
  foreach ($l in $kept) {
    $buf = if ($buf) { "$buf $l" } else { $l }
    if ($buf -match '[.!?:)"”…~\p{So}]$') { $paras += $buf; $buf = '' }
  }
  if ($buf) { $paras += $buf }
  $o = [ordered]@{ name = $p.name; photo = $p.photo; bio = $paras }
  if ($sl) { $o.sl = $sl }
  if ($scheduleOverrides.ContainsKey($p.name)) { $o.schedule = $scheduleOverrides[$p.name] }
  elseif ($schedule.Count) { $o.schedule = $schedule }
  return $o
}

$out = [ordered]@{
  djs   = @($data.djs   | ForEach-Object { Clean-Person $_ })
  hosts = @($data.hosts | ForEach-Object { Clean-Person $_ })
}
$json = $out | ConvertTo-Json -Depth 5
$js = "// Staff profiles, imported from the old Wix site. Edit here to add or update a DJ or host.`nwindow.MUDDYS_STAFF = $json;`n"
[IO.File]::WriteAllText("$site\staff-data.js", $js, (New-Object System.Text.UTF8Encoding $false))
$out.hosts | Where-Object { $_.name -in 'Belona', 'Kelly', 'Life Suki', 'Sophia' } | ConvertTo-Json -Depth 4
$out.djs | Where-Object { $_.name -in 'DJ Andy', 'DJ John' } | ForEach-Object { $_.name; $_.bio | ForEach-Object { "  - " + $_.Substring(0, [Math]::Min(90, $_.Length)) } }
