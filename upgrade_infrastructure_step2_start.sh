# Copyright 2018 ETH Zurich
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

# check we have $SC, and gen folder
if [ ! -d "$SC" ]; then
    echo "SC folder not found. SC=$SC"
    exit 1
fi
cd $SC
if [ ! -d "gen" ]; then
    echo "gen folder not found"
    exit 1
fi
if [ ! -d "gen_nextversion" ]; then
    echo "gen_nextversion folder not found"
    exit 1
fi

mv gen gen.bk`date +%Y%m%d%H%M`
mv gen_nextversion gen

mv gen-cache gen-cache.bk`date +%Y%m%d%H%M`
mkdir gen-cache

exit 1
output=$(./tools/zkcleanslate --zk 127.0.0.1:2181 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "Zookeeper cleanslate failed:"
    echo "$output"
    # exit 1
fi
output=$(./scion.sh run nobuild 2>&1) && success=1 || success=0
if [ $success != 1 ]; then
    echo "SCION run failed:"
    echo "$output"
    # exit 1
fi
