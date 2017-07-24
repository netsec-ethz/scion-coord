angular.module('scionApp')
    .controller('adminCtrl', ['$scope', 'adminService', '$location', '$window',
        function($scope, adminService, $location, $window) {

            $scope.me = function() {

                adminService.me().then(
                    function(data) {
                        console.log(data);
                        $scope.user = data;
                    },
                    function(response) {
                        console.log(response);
                        //$location.path('/');
                    });
            };


            // refresh the data when the controller is loaded
            $scope.me();


     }
    ]);
