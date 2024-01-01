bin/darwin_arm64/syncAlbum -l|cut -c 12-47|while IFS="" read -r p || [ -n "$p" ]
do
  trimmed_string=$(echo $p | sed 's/ *$//')
  printf '%s\n' " $trimmed_string"
   bin/darwin_arm64/syncAlbum -a "$trimmed_string"
done

