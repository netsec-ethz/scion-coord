scionApp
    .controller('resendCtrl', ['$rootScope', '$scope', 'resendService', function($rootScope, $scope, resendService) {

        $scope.email = $rootScope.resendAddress;
        $rootScope.resendAddress = ""

        $scope.resendEmail = function(email){

                resendService.resendEmail(email).then(function (response){
                    $scope.message = "Verification email sent to " + email;
                    $scope.error = "";
                },
                function(response){
                    $scope.message = "";
                    $scope.error = response.data;
                });
                $scope.email = "";
        };

}]);
